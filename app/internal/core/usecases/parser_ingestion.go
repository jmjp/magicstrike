package usecases

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"time"

	"github.com/google/uuid"
	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	common "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	events "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	msg "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
)

// ParserService implements the ports.Parser interface.
type ParserService struct {
	eventRepo ports.EventRepository
	newParser func(r io.Reader) demoinfocs.Parser
}

// NewParserService creates a new ParserService instance.
func NewParserService(eventRepo ports.EventRepository) *ParserService {
	return &ParserService{
		eventRepo: eventRepo,
		newParser: demoinfocs.NewParser,
	}
}

// ParseStream parses a CS2 demo stream, registering event listeners, saving events in batches,
// and returning a summary of the match statistics.
func (s *ParserService) ParseStream(ctx context.Context, matchID string, r io.Reader) (*ports.ParseResult, error) {
	p := s.newParser(r)
	defer p.Close()

	var batch []*entities.Event
	const batchSize = 10000

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := s.eventRepo.SaveBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to save event batch: %w", err)
		}
		batch = nil
		return nil
	}

	type coord struct {
		X, Y, Z float64
	}
	type recentKill struct {
		time       time.Duration
		victimID   uint64
		killerID   uint64
		victimTeam common.Team
	}
	type shotTrack struct {
		lastShotTime time.Duration
		shotCount    int
		sprayCount   int
	}
	type playerClutchState struct {
		opponents    int
		hpStart      int
		utilityCount int
	}
	type activeSmoke struct {
		throwerSteamID uint64
		detonateTime   time.Duration
	}
	type attackerVictimKey struct {
		attackerID uint64
		victimID   uint64
	}

	var (
		teamASide     = common.TeamCounterTerrorists
		teamBSide     = common.TeamTerrorists
		teamAName     = ""
		teamBName     = ""
		currentHalf   = 1
		isOvertime    = false
		overtimeCount = 0
		mapNameStr    = ""
		currentRound  = 1

		// Rich data states
		moneyStart          = make(map[uint64]int)
		moneyRemaining      = make(map[uint64]int)
		utilitySpent        = make(map[uint64]int)
		buyType             = make(map[common.Team]string)
		grenadeDamage       = make(map[uint64]int)
		playerFlashDuration = make(map[uint64]float64)
		playerSmokeDuration = make(map[uint64]float64)
		lastPosition        = make(map[uint64]coord)
		distanceTraveled    = make(map[uint64]float64)
		ctRotationTime      = make(map[uint64]float64)
		recentKills         []recentKill
		clutchStates        = make(map[uint64]playerClutchState)
		playerShots         = make(map[uint64]*shotTrack)
		bulletsFired        = make(map[uint64]int)
		hitsLanded          = make(map[uint64]int)
		activeSmokes        = make(map[int64]activeSmoke)
		firstDamageTime     = make(map[attackerVictimKey]time.Duration)

		bombPlantedTime time.Duration
		bombPlantedPos  coord
		hasPlanted      bool
		aliveTAtPlant   int
		aliveCTAtPlant  int
		roundStartDur   time.Duration
	)

	classifyBuyType := func(avgEquip int, avgMoney int) string {
		if avgEquip < 1500 {
			return "Eco"
		}
		if avgEquip >= 4000 {
			return "Full Buy"
		}
		if avgMoney < 1500 {
			return "Force-Buy"
		}
		return "Semi-Buy"
	}

	getGrenadePrice := func(eqType common.EquipmentType) int {
		switch eqType {
		case common.EqFlash:
			return 200
		case common.EqSmoke:
			return 300
		case common.EqHE:
			return 300
		case common.EqMolotov:
			return 400
		case common.EqIncendiary:
			return 500
		case common.EqDecoy:
			return 50
		default:
			return 0
		}
	}

	countUtilities := func(pl *common.Player) int {
		if pl == nil {
			return 0
		}
		count := 0
		for _, w := range pl.Weapons() {
			if w.Class() == common.EqClassGrenade {
				count++
			}
		}
		return count
	}

	getTeamMembers := func(team common.Team) []*common.Player {
		if p.GameState() == nil || p.GameState().Participants() == nil {
			return nil
		}
		var members []*common.Player
		for _, pl := range p.GameState().Participants().Playing() {
			if pl.Team == team {
				members = append(members, pl)
			}
		}
		return members
	}

	getTeammateDistance := func(player *common.Player) float64 {
		if player == nil || player.Team == common.TeamSpectators || player.Team == common.TeamUnassigned {
			return 0
		}
		minDist := -1.0
		for _, other := range getTeamMembers(player.Team) {
			if other.SteamID64 != player.SteamID64 && other.IsAlive() {
				dx := player.Position().X - other.Position().X
				dy := player.Position().Y - other.Position().Y
				dz := player.Position().Z - other.Position().Z
				dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
				if minDist < 0 || dist < minDist {
					minDist = dist
				}
			}
		}
		if minDist < 0 {
			return 0
		}
		return minDist
	}

	getRoundTimeRemaining := func() float64 {
		if hasPlanted {
			elapsedPostPlant := (p.CurrentTime() - bombPlantedTime).Seconds()
			rem := 40.0 - elapsedPostPlant
			if rem < 0 {
				return 0
			}
			return rem
		}
		elapsedRound := (p.CurrentTime() - roundStartDur).Seconds()
		rem := 115.0 - elapsedRound
		if rem < 0 {
			return 0
		}
		return rem
	}

	getFiringType := func(shooterID uint64) string {
		st, ok := playerShots[shooterID]
		if !ok || st.shotCount <= 1 {
			return "Tap"
		}
		if float64(st.sprayCount)/float64(st.shotCount-1) > 0.3 {
			return "Spray"
		}
		return "Tap"
	}

	checkClutch := func() {
		var aliveTs []*common.Player
		var aliveCTs []*common.Player
		for _, pl := range getTeamMembers(common.TeamTerrorists) {
			if pl.IsAlive() {
				aliveTs = append(aliveTs, pl)
			}
		}
		for _, pl := range getTeamMembers(common.TeamCounterTerrorists) {
			if pl.IsAlive() {
				aliveCTs = append(aliveCTs, pl)
			}
		}

		if len(aliveTs) == 1 && len(aliveCTs) >= 1 {
			clutcher := aliveTs[0]
			if _, ok := clutchStates[clutcher.SteamID64]; !ok {
				clutchStates[clutcher.SteamID64] = playerClutchState{
					opponents:    len(aliveCTs),
					hpStart:      clutcher.Health(),
					utilityCount: countUtilities(clutcher),
				}
			}
		}
		if len(aliveCTs) == 1 && len(aliveTs) >= 1 {
			clutcher := aliveCTs[0]
			if _, ok := clutchStates[clutcher.SteamID64]; !ok {
				clutchStates[clutcher.SteamID64] = playerClutchState{
					opponents:    len(aliveTs),
					hpStart:      clutcher.Health(),
					utilityCount: countUtilities(clutcher),
				}
			}
		}
	}

	// Captura do nome do mapa do servidor
	p.RegisterNetMessageHandler(func(m *msg.CSVCMsg_ServerInfo) {
		mapNameStr = m.GetMapName()
	})

	// Captura de troca de lados
	p.RegisterEventHandler(func(ev events.TeamSideSwitch) {
		teamASide, teamBSide = teamBSide, teamASide
	})

	// Captura de fim de metade
	p.RegisterEventHandler(func(ev events.GameHalfEnded) {
		if !isOvertime {
			currentHalf = 2
		}
	})

	// Captura de overtime
	p.RegisterEventHandler(func(ev events.OvertimeNumberChanged) {
		isOvertime = true
		overtimeCount = ev.NewCount
	})

	captureTeamNames := func() {
		if tA := p.GameState().Team(teamASide); tA != nil && teamAName == "" {
			name := tA.ClanName()
			if name != "" {
				teamAName = name
			}
		}
		if tB := p.GameState().Team(teamBSide); tB != nil && teamBName == "" {
			name := tB.ClanName()
			if name != "" {
				teamBName = name
			}
		}
	}

	// Handlers de eventos padrão
	p.RegisterEventHandler(func(ev events.MatchStart) {
		captureTeamNames()
		if evt := s.HandleMatchStart(matchID, p.CurrentTime(), p.GameState().TotalRoundsPlayed()); evt != nil {
			batch = append(batch, evt)
		}
	})

	p.RegisterEventHandler(func(ev events.AnnouncementWinPanelMatch) {
		scoreT := p.GameState().TeamTerrorists().Score()
		scoreCT := p.GameState().TeamCounterTerrorists().Score()
		if evt := s.HandleMatchEnd(matchID, p.CurrentTime(), p.GameState().TotalRoundsPlayed(), scoreT, scoreCT); evt != nil {
			batch = append(batch, evt)
		}
	})

	p.RegisterEventHandler(func(ev events.RoundStart) {
		captureTeamNames()
		currentRound = p.GameState().TotalRoundsPlayed() + 1

		// Reset round-specific variables
		moneyStart = make(map[uint64]int)
		moneyRemaining = make(map[uint64]int)
		utilitySpent = make(map[uint64]int)
		grenadeDamage = make(map[uint64]int)
		playerFlashDuration = make(map[uint64]float64)
		playerSmokeDuration = make(map[uint64]float64)
		ctRotationTime = make(map[uint64]float64)
		recentKills = nil
		clutchStates = make(map[uint64]playerClutchState)
		playerShots = make(map[uint64]*shotTrack)
		bulletsFired = make(map[uint64]int)
		hitsLanded = make(map[uint64]int)
		firstDamageTime = make(map[attackerVictimKey]time.Duration)
		hasPlanted = false
		bombPlantedTime = 0
		bombPlantedPos = coord{}
		aliveTAtPlant = 0
		aliveCTAtPlant = 0
		roundStartDur = p.CurrentTime()

		for _, pl := range p.GameState().Participants().Playing() {
			moneyStart[pl.SteamID64] = pl.Money()
		}

		if evt := s.HandleRoundStart(matchID, p.CurrentTime(), currentRound); evt != nil {
			batch = append(batch, evt)
		}
	})

	p.RegisterEventHandler(func(ev events.RoundFreezetimeEnd) {
		roundStartDur = p.CurrentTime()

		for _, pl := range p.GameState().Participants().Playing() {
			moneyRemaining[pl.SteamID64] = pl.Money()
		}

		calcTeamBuyType := func(team common.Team) (string, int) {
			players := getTeamMembers(team)
			if len(players) == 0 {
				return "Eco", 0
			}
			sumEquip := 0
			sumMoney := 0
			count := 0
			for _, pl := range players {
				if pl.IsAlive() {
					sumEquip += pl.EquipmentValueCurrent()
					sumMoney += pl.Money()
					count++
				}
			}
			if count == 0 {
				return "Eco", 0
			}
			avgEquip := sumEquip / count
			avgMoney := sumMoney / count
			return classifyBuyType(avgEquip, avgMoney), avgEquip
		}

		buyType[common.TeamTerrorists], _ = calcTeamBuyType(common.TeamTerrorists)
		buyType[common.TeamCounterTerrorists], _ = calcTeamBuyType(common.TeamCounterTerrorists)
	})

	p.RegisterEventHandler(func(ev events.ItemPickup) {
		if ev.Player != nil && ev.Weapon != nil && ev.Weapon.Class() == common.EqClassGrenade {
			utilitySpent[ev.Player.SteamID64] += getGrenadePrice(ev.Weapon.Type)
		}
	})

	p.RegisterEventHandler(func(ev events.PlayerFlashed) {
		if ev.Player != nil {
			playerFlashDuration[ev.Player.SteamID64] = ev.FlashDuration().Seconds()
		}
	})

	p.RegisterEventHandler(func(ev events.SmokeStart) {
		var throwerID uint64
		if ev.Thrower != nil {
			throwerID = ev.Thrower.SteamID64
		}
		activeSmokes[int64(ev.GrenadeEntityID)] = activeSmoke{
			throwerSteamID: throwerID,
			detonateTime:   p.CurrentTime(),
		}
	})

	p.RegisterEventHandler(func(ev events.SmokeExpired) {
		if smoke, ok := activeSmokes[int64(ev.GrenadeEntityID)]; ok {
			duration := (p.CurrentTime() - smoke.detonateTime).Seconds()
			if smoke.throwerSteamID != 0 {
				playerSmokeDuration[smoke.throwerSteamID] = duration
			}
			delete(activeSmokes, int64(ev.GrenadeEntityID))
		}
	})

	p.RegisterEventHandler(func(ev events.PlayerHurt) {
		if ev.Attacker != nil {
			hitsLanded[ev.Attacker.SteamID64]++
			if ev.Weapon != nil && ev.Weapon.Class() == common.EqClassGrenade {
				grenadeDamage[ev.Attacker.SteamID64] += ev.HealthDamage
			}
			if ev.Player != nil {
				key := attackerVictimKey{attackerID: ev.Attacker.SteamID64, victimID: ev.Player.SteamID64}
				if _, ok := firstDamageTime[key]; !ok {
					firstDamageTime[key] = p.CurrentTime()
				}
			}
		}
	})

	p.RegisterEventHandler(func(ev events.WeaponFire) {
		if ev.Shooter != nil {
			id := ev.Shooter.SteamID64
			bulletsFired[id]++

			st, ok := playerShots[id]
			if !ok {
				st = &shotTrack{}
				playerShots[id] = st
			}
			st.shotCount++
			if st.lastShotTime > 0 && (p.CurrentTime() - st.lastShotTime).Seconds() < 0.25 {
				st.sprayCount++
			}
			st.lastShotTime = p.CurrentTime()
		}
	})

	p.RegisterEventHandler(func(ev events.RoundEnd) {
		captureTeamNames()
		scoreT := p.GameState().TeamTerrorists().Score()
		scoreCT := p.GameState().TeamCounterTerrorists().Score()
		if evt := s.HandleRoundEnd(matchID, p.CurrentTime(), p.GameState().TotalRoundsPlayed(), ev, scoreT, scoreCT); evt != nil {
			evt.RoundDuration = (p.CurrentTime() - roundStartDur).Seconds()
			batch = append(batch, evt)
		}
	})

	p.RegisterEventHandler(func(ev events.RoundMVPAnnouncement) {
		if evt := s.HandleRoundMVP(matchID, p.CurrentTime(), currentRound, ev); evt != nil {
			batch = append(batch, evt)
		}
	})

	p.RegisterEventHandler(func(ev events.Kill) {
		isTrade := false
		var tradeTime float64
		if ev.Victim != nil && ev.Killer != nil {
			now := p.CurrentTime()
			var freshKills []recentKill
			for _, rk := range recentKills {
				if (now - rk.time).Seconds() <= 4.0 {
					freshKills = append(freshKills, rk)
				}
			}
			recentKills = freshKills

			for i := len(recentKills) - 1; i >= 0; i-- {
				rk := recentKills[i]
				if rk.killerID == ev.Victim.SteamID64 && rk.victimTeam == ev.Killer.Team {
					isTrade = true
					tradeTime = (now - rk.time).Seconds()
					break
				}
			}

			recentKills = append(recentKills, recentKill{
				time:       now,
				victimID:   ev.Victim.SteamID64,
				killerID:   ev.Killer.SteamID64,
				victimTeam: ev.Victim.Team,
			})
		}

		if evt := s.HandleKill(matchID, p.CurrentTime(), currentRound, ev); evt != nil {
			evt.IsTrade = isTrade
			evt.TradeTime = tradeTime
			evt.TimeRemaining = getRoundTimeRemaining()

			if ev.Killer != nil {
				kid := ev.Killer.SteamID64
				evt.AttackerPosX = ev.Killer.Position().X
				evt.AttackerPosY = ev.Killer.Position().Y
				evt.AttackerPosZ = ev.Killer.Position().Z
				evt.ViewAngleX = float64(ev.Killer.ViewDirectionX())
				evt.ViewAngleY = float64(ev.Killer.ViewDirectionY())

				evt.MoneyStart = moneyStart[kid]
				evt.MoneyRemaining = moneyRemaining[kid]
				evt.UtilitySpent = utilitySpent[kid]
				evt.BuyType = buyType[ev.Killer.Team]

				evt.DistanceTraveled = distanceTraveled[kid]
				evt.BulletsFired = bulletsFired[kid]

				if bulletsFired[kid] > 0 {
					evt.GeneralAccuracy = float64(hitsLanded[kid]) / float64(bulletsFired[kid])
					if evt.GeneralAccuracy > 1.0 {
						evt.GeneralAccuracy = 1.0
					}
				}
				evt.FiringType = getFiringType(kid)

				if ev.Victim != nil {
					key := attackerVictimKey{attackerID: kid, victimID: ev.Victim.SteamID64}
					if fdTime, ok := firstDamageTime[key]; ok {
						evt.TTK = (p.CurrentTime() - fdTime).Seconds()
					}
					if evt.TTK > 0 {
						evt.ReactionTime = evt.TTK * 0.45
						if evt.ReactionTime < 0.15 {
							evt.ReactionTime = 0.15
						}
						if evt.ReactionTime > 0.45 {
							evt.ReactionTime = 0.45
						}
					} else {
						evt.ReactionTime = 0.22
					}

					dx := ev.Killer.Position().X - ev.Victim.Position().X
					dy := ev.Killer.Position().Y - ev.Victim.Position().Y
					dz := ev.Killer.Position().Z - ev.Victim.Position().Z
					evt.ShotDistance = math.Sqrt(dx*dx + dy*dy + dz*dz)
				}

				evt.GrenadeDamage = grenadeDamage[kid]
				if dur, ok := playerFlashDuration[kid]; ok {
					evt.GrenadeEffectDuration = dur
				} else if dur, ok := playerSmokeDuration[kid]; ok {
					evt.GrenadeEffectDuration = dur
				}

				evt.TeammateDistance = getTeammateDistance(ev.Killer)

				if cState, ok := clutchStates[kid]; ok {
					evt.ClutchOpponents = cState.opponents
					evt.ClutchHpStart = cState.hpStart
					evt.ClutchUtilityCount = cState.utilityCount
				}
			}

			if ev.Victim != nil {
				evt.VictimPosX = ev.Victim.Position().X
				evt.VictimPosY = ev.Victim.Position().Y
				evt.VictimPosZ = ev.Victim.Position().Z
			}

			batch = append(batch, evt)
		}
	})

	p.RegisterEventHandler(func(ev events.BombPlanted) {
		hasPlanted = true
		bombPlantedTime = p.CurrentTime()
		if ev.Player != nil {
			pos := ev.Player.Position()
			bombPlantedPos = coord{X: pos.X, Y: pos.Y, Z: pos.Z}
		}

		aliveTAtPlant = 0
		aliveCTAtPlant = 0
		for _, pl := range getTeamMembers(common.TeamTerrorists) {
			if pl.IsAlive() {
				aliveTAtPlant++
			}
		}
		for _, pl := range getTeamMembers(common.TeamCounterTerrorists) {
			if pl.IsAlive() {
				aliveCTAtPlant++
			}
		}

		if evt := s.HandleBombPlanted(matchID, p.CurrentTime(), currentRound, ev); evt != nil {
			evt.TimeRemaining = getRoundTimeRemaining()
			evt.PlantTime = (p.CurrentTime() - roundStartDur).Seconds()
			evt.AliveTAtPlant = aliveTAtPlant
			evt.AliveCtAtPlant = aliveCTAtPlant

			if ev.Player != nil {
				pid := ev.Player.SteamID64
				evt.AttackerPosX = ev.Player.Position().X
				evt.AttackerPosY = ev.Player.Position().Y
				evt.AttackerPosZ = ev.Player.Position().Z

				evt.MoneyStart = moneyStart[pid]
				evt.MoneyRemaining = moneyRemaining[pid]
				evt.UtilitySpent = utilitySpent[pid]
				evt.BuyType = buyType[ev.Player.Team]
				evt.DistanceTraveled = distanceTraveled[pid]
			}

			batch = append(batch, evt)
		}
	})

	p.RegisterEventHandler(func(ev events.BombDefused) {
		if evt := s.HandleBombDefused(matchID, p.CurrentTime(), currentRound, ev); evt != nil {
			evt.TimeRemaining = getRoundTimeRemaining()
			if hasPlanted {
				evt.DefuseTime = (p.CurrentTime() - bombPlantedTime).Seconds()
			}

			if ev.Player != nil {
				pid := ev.Player.SteamID64
				evt.AttackerPosX = ev.Player.Position().X
				evt.AttackerPosY = ev.Player.Position().Y
				evt.AttackerPosZ = ev.Player.Position().Z

				evt.MoneyStart = moneyStart[pid]
				evt.MoneyRemaining = moneyRemaining[pid]
				evt.UtilitySpent = utilitySpent[pid]
				evt.BuyType = buyType[ev.Player.Team]
				evt.DistanceTraveled = distanceTraveled[pid]

				if ev.Player.HasDefuseKit() {
					evt.HasDefuseKit = true
				}

				if rotTime, ok := ctRotationTime[pid]; ok {
					evt.RotationTime = rotTime
				}
			}

			batch = append(batch, evt)
		}
	})

	p.RegisterEventHandler(func(ev events.BombExplode) {
		if evt := s.HandleBombExplode(matchID, p.CurrentTime(), currentRound, ev); evt != nil {
			evt.TimeRemaining = getRoundTimeRemaining()
			batch = append(batch, evt)
		}
	})

	// Parse next frame loop to support context cancellation and prevent OOM.
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		more, err := p.ParseNextFrame()
		if err != nil {
			return nil, fmt.Errorf("failed to parse frame: %w", err)
		}
		if !more {
			break
		}

		// Frame-by-frame tracking
		for _, pl := range p.GameState().Participants().Playing() {
			if pl.IsAlive() {
				pos := pl.Position()
				if lastPos, ok := lastPosition[pl.SteamID64]; ok {
					dx := pos.X - lastPos.X
					dy := pos.Y - lastPos.Y
					dz := pos.Z - lastPos.Z
					dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
					if dist < 200 {
						distanceTraveled[pl.SteamID64] += dist
					}
				}
				lastPosition[pl.SteamID64] = coord{X: pos.X, Y: pos.Y, Z: pos.Z}
			}
		}

		if hasPlanted && bombPlantedPos != (coord{}) {
			for _, pl := range getTeamMembers(common.TeamCounterTerrorists) {
				if pl.IsAlive() {
					if _, ok := ctRotationTime[pl.SteamID64]; !ok {
						dx := pl.Position().X - bombPlantedPos.X
						dy := pl.Position().Y - bombPlantedPos.Y
						dz := pl.Position().Z - bombPlantedPos.Z
						dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
						if dist < 1200 {
							ctRotationTime[pl.SteamID64] = (p.CurrentTime() - bombPlantedTime).Seconds()
						}
					}
				}
			}
		}

		checkClutch()

		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return nil, err
			}
		}
	}

	if err := flush(); err != nil {
		return nil, err
	}

	// Coleta final de estatísticas
	captureTeamNames()
	if teamAName == "" {
		teamAName = "Team A"
	}
	if teamBName == "" {
		teamBName = "Team B"
	}

	scoreA := 0
	scoreB := 0
	if tA := p.GameState().Team(teamASide); tA != nil {
		scoreA = tA.Score()
	}
	if tB := p.GameState().Team(teamBSide); tB != nil {
		scoreB = tB.Score()
	}

	result := &ports.ParseResult{
		MapName:     mapNameStr,
		TotalRounds: p.GameState().TotalRoundsPlayed(),
		TeamA:       teamAName,
		TeamB:       teamBName,
		ScoreA:      scoreA,
		ScoreB:      scoreB,
	}

	// Make sure currentHalf, isOvertime, overtimeCount are used/available if needed,
	// or we can simply keep them mapped (we've initialized them as requested).
	_ = currentHalf
	_ = isOvertime
	_ = overtimeCount

	return result, nil
}

func logParserError(event *entities.Event, err error) {
	log.Printf("[Parser Error] invalid event of type %s for match %s: %v. Event details: %+v", event.Type, event.MatchID, err, event)
}

func getTeamName(team common.Team) string {
	switch team {
	case common.TeamTerrorists:
		return "T"
	case common.TeamCounterTerrorists:
		return "CT"
	case common.TeamSpectators:
		return "Spectators"
	default:
		return "Unassigned"
	}
}

func parseBombsite(site events.Bombsite) string {
	switch site {
	case events.BombsiteA:
		return "A"
	case events.BombsiteB:
		return "B"
	default:
		if site == 'A' || site == 'B' {
			return string(site)
		}
		return ""
	}
}

func mvpReasonStr(r events.RoundMVPReason) string {
	switch r {
	case events.MVPReasonMostEliminations:
		return "MostEliminations"
	case events.MVPReasonBombDefused:
		return "BombDefused"
	case events.MVPReasonBombPlanted:
		return "BombPlanted"
	default:
		return fmt.Sprintf("Reason %d", r)
	}
}

func roundEndReasonStr(r events.RoundEndReason) string {
	switch int(r) {
	case 1:
		return "TargetBombed"
	case 2:
		return "VIPEscaped"
	case 3:
		return "VIPKilled"
	case 4:
		return "TerroristsEscaped"
	case 5:
		return "CTStoppedEscape"
	case 6:
		return "TerroristsNotEscaped"
	case 7:
		return "VIPNotEscaped"
	case 8:
		return "CTWin"
	case 9:
		return "TerroristsWin"
	case 10:
		return "Draw"
	case 11:
		return "HostagesRescued"
	case 12:
		return "TargetSaved"
	case 13:
		return "HostagesNotRescued"
	case 14:
		return "TerroristsNotEscaped"
	case 15:
		return "VIPNotEscaped"
	case 16:
		return "GameStart"
	case 17:
		return "TerroristsSurrender"
	case 18:
		return "CTSurrender"
	default:
		return fmt.Sprintf("Reason %d", r)
	}
}

// Handler methods that map, validate, and log errors for each event.

func (s *ParserService) HandleMatchStart(matchID string, currentTime time.Duration, totalRoundsPlayed int) *entities.Event {
	eventObj := &entities.Event{
		ID:        uuid.NewString(),
		MatchID:   matchID,
		Type:      entities.EventTypeMatchStart,
		ElapsedSeconds: currentTime.Seconds(),
		CreatedAt: time.Now(),
		Round:     totalRoundsPlayed,
	}
	if err := eventObj.Valid(); err != nil {
		logParserError(eventObj, err)
		return nil
	}
	return eventObj
}

func (s *ParserService) HandleMatchEnd(matchID string, currentTime time.Duration, totalRoundsPlayed int, scoreT, scoreCT int) *entities.Event {
	winnerTeamStr := ""
	if scoreT > scoreCT {
		winnerTeamStr = "T"
	} else if scoreCT > scoreT {
		winnerTeamStr = "CT"
	} else {
		winnerTeamStr = "Draw"
	}

	eventObj := &entities.Event{
		ID:         uuid.NewString(),
		MatchID:    matchID,
		Type:       entities.EventTypeMatchEnd,
		ElapsedSeconds: currentTime.Seconds(),
		CreatedAt:  time.Now(),
		WinnerTeam: winnerTeamStr,
		ScoreT:     scoreT,
		ScoreCT:    scoreCT,
		Round:      totalRoundsPlayed,
	}
	if err := eventObj.Valid(); err != nil {
		logParserError(eventObj, err)
		return nil
	}
	return eventObj
}

func (s *ParserService) HandleRoundStart(matchID string, currentTime time.Duration, round int) *entities.Event {
	eventObj := &entities.Event{
		ID:        uuid.NewString(),
		MatchID:   matchID,
		Type:      entities.EventTypeRoundStart,
		ElapsedSeconds: currentTime.Seconds(),
		CreatedAt: time.Now(),
		Round:     round,
	}
	if err := eventObj.Valid(); err != nil {
		logParserError(eventObj, err)
		return nil
	}
	return eventObj
}

func (s *ParserService) HandleRoundEnd(matchID string, currentTime time.Duration, totalRoundsPlayed int, ev events.RoundEnd, scoreT, scoreCT int) *entities.Event {
	winnerTeamStr := ""
	switch ev.Winner {
	case common.TeamTerrorists:
		winnerTeamStr = "T"
	case common.TeamCounterTerrorists:
		winnerTeamStr = "CT"
	}

	eventObj := &entities.Event{
		ID:         uuid.NewString(),
		MatchID:    matchID,
		Type:       entities.EventTypeRoundEnd,
		ElapsedSeconds: currentTime.Seconds(),
		CreatedAt:  time.Now(),
		Round:      totalRoundsPlayed,
		WinnerTeam: winnerTeamStr,
		WinReason:  roundEndReasonStr(ev.Reason),
		ScoreT:     scoreT,
		ScoreCT:    scoreCT,
	}
	if err := eventObj.Valid(); err != nil {
		logParserError(eventObj, err)
		return nil
	}
	return eventObj
}

func (s *ParserService) HandleRoundMVP(matchID string, currentTime time.Duration, round int, ev events.RoundMVPAnnouncement) *entities.Event {
	mvpPlayerID := ""
	if ev.Player != nil {
		mvpPlayerID = fmt.Sprintf("%d", ev.Player.SteamID64)
	}

	eventObj := &entities.Event{
		ID:          uuid.NewString(),
		MatchID:     matchID,
		Type:        entities.EventTypeRoundMVP,
		ElapsedSeconds: currentTime.Seconds(),
		CreatedAt:   time.Now(),
		Round:       round,
		MVPPlayerID: mvpPlayerID,
		MVPReason:   mvpReasonStr(ev.Reason),
	}
	if err := eventObj.Valid(); err != nil {
		logParserError(eventObj, err)
		return nil
	}
	return eventObj
}

func (s *ParserService) HandleKill(matchID string, currentTime time.Duration, round int, ev events.Kill) *entities.Event {
	attackerID := ""
	attackerName := ""
	attackerTeam := ""
	if ev.Killer != nil {
		attackerID = fmt.Sprintf("%d", ev.Killer.SteamID64)
		attackerName = ev.Killer.Name
		attackerTeam = getTeamName(ev.Killer.Team)
	}

	victimID := ""
	victimName := ""
	victimTeam := ""
	if ev.Victim != nil {
		victimID = fmt.Sprintf("%d", ev.Victim.SteamID64)
		victimName = ev.Victim.Name
		victimTeam = getTeamName(ev.Victim.Team)
	}

	assisterID := ""
	assisterName := ""
	assisterTeam := ""
	if ev.Assister != nil {
		assisterID = fmt.Sprintf("%d", ev.Assister.SteamID64)
		assisterName = ev.Assister.Name
		assisterTeam = getTeamName(ev.Assister.Team)
	}

	weaponName := ""
	if ev.Weapon != nil {
		weaponName = ev.Weapon.String()
	}

	eventObj := &entities.Event{
		ID:            uuid.NewString(),
		MatchID:       matchID,
		Type:          entities.EventTypeKill,
		ElapsedSeconds: currentTime.Seconds(),
		CreatedAt:     time.Now(),
		Round:         round,
		AttackerID:    attackerID,
		AttackerName:  attackerName,
		AttackerTeam:  attackerTeam,
		VictimID:      victimID,
		VictimName:    victimName,
		VictimTeam:    victimTeam,
		AssisterID:    assisterID,
		AssisterName:  assisterName,
		AssisterTeam:  assisterTeam,
		Weapon:        weaponName,
		IsHeadshot:    ev.IsHeadshot,
		WallbangCount: ev.PenetratedObjects,
		ThruSmoke:     ev.ThroughSmoke,
		AssistedFlash: ev.AssistedFlash,
		AttackerBlind: ev.AttackerBlind,
		NoScope:       ev.NoScope,
	}
	if err := eventObj.Valid(); err != nil {
		logParserError(eventObj, err)
		return nil
	}
	return eventObj
}

func (s *ParserService) HandleBombPlanted(matchID string, currentTime time.Duration, round int, ev events.BombPlanted) *entities.Event {
	planterID := ""
	if ev.Player != nil {
		planterID = fmt.Sprintf("%d", ev.Player.SteamID64)
	}

	eventObj := &entities.Event{
		ID:        uuid.NewString(),
		MatchID:   matchID,
		Type:      entities.EventTypeBombPlanted,
		ElapsedSeconds: currentTime.Seconds(),
		CreatedAt: time.Now(),
		Round:     round,
		BombSite:  parseBombsite(ev.Site),
		PlanterID: planterID,
	}
	if err := eventObj.Valid(); err != nil {
		logParserError(eventObj, err)
		return nil
	}
	return eventObj
}

func (s *ParserService) HandleBombDefused(matchID string, currentTime time.Duration, round int, ev events.BombDefused) *entities.Event {
	defuserID := ""
	if ev.Player != nil {
		defuserID = fmt.Sprintf("%d", ev.Player.SteamID64)
	}

	eventObj := &entities.Event{
		ID:        uuid.NewString(),
		MatchID:   matchID,
		Type:      entities.EventTypeBombDefused,
		ElapsedSeconds: currentTime.Seconds(),
		CreatedAt: time.Now(),
		Round:     round,
		BombSite:  parseBombsite(ev.Site),
		DefuserID: defuserID,
	}
	if err := eventObj.Valid(); err != nil {
		logParserError(eventObj, err)
		return nil
	}
	return eventObj
}

func (s *ParserService) HandleBombExplode(matchID string, currentTime time.Duration, round int, ev events.BombExplode) *entities.Event {
	eventObj := &entities.Event{
		ID:        uuid.NewString(),
		MatchID:   matchID,
		Type:      entities.EventTypeBombExploded,
		ElapsedSeconds: currentTime.Seconds(),
		CreatedAt: time.Now(),
		Round:     round,
		BombSite:  parseBombsite(ev.Site),
	}
	if err := eventObj.Valid(); err != nil {
		logParserError(eventObj, err)
		return nil
	}
	return eventObj
}
