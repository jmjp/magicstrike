package cs2dialect

// CS2Term representa um termo do dialeto de CS2 com seu significado e contexto.
type CS2Term struct {
	Term     string   // O termo em si
	Meaning  string   // Significado em português
	Category string   // Categoria (giria, posicao, tatica, comunicacao, arma, funcao)
	Example  string   // Exemplo de uso em jogo
	Aliases  []string // Variações ou termos alternativos
}

// CS2Lexicon contém todo o léxico do CS2.
type CS2Lexicon struct {
	terms map[string]CS2Term
}

// NewCS2Lexicon inicializa e retorna o léxico completo do CS2.
func NewCS2Lexicon() *CS2Lexicon {
	l := &CS2Lexicon{
		terms: make(map[string]CS2Term),
	}
	l.load()
	return l
}

// Lookup busca um termo no léxico. Retorna o termo e true se encontrado.
func (l *CS2Lexicon) Lookup(term string) (CS2Term, bool) {
	t, ok := l.terms[normalize(term)]
	return t, ok
}

// LookupFuzzy busca um termo e seus aliases. Retorna nil se nada for encontrado.
func (l *CS2Lexicon) LookupFuzzy(input string) []CS2Term {
	key := normalize(input)
	var results []CS2Term
	for k, t := range l.terms {
		if k == key {
			results = append(results, t)
			continue
		}
		for _, alias := range t.Aliases {
			if normalize(alias) == key {
				results = append(results, t)
				break
			}
		}
	}
	return results
}

// AllByCategory retorna todos os termos de uma categoria específica.
// Categorias: "giria", "posicao", "tatica", "comunicacao", "arma", "funcao", "economia"
func (l *CS2Lexicon) AllByCategory(category string) []CS2Term {
	var result []CS2Term
	for _, t := range l.terms {
		if t.Category == category {
			result = append(result, t)
		}
	}
	return result
}

// GetSystemPromptContext retorna um texto formatado para uso como contexto de sistema
// num agente de IA, descrevendo o dialeto do CS2 de forma estruturada.
func (l *CS2Lexicon) GetSystemPromptContext() string {
	return `Você entende o dialeto e as gírias de Counter-Strike 2 (CS2). 
Abaixo está o vocabulário completo que você conhece:

=== GÍRIAS E COMUNICAÇÃO ===
ace/neide: matar todos os 5 inimigos na mesma rodada
bait/baiter: usar um aliado como isca para conseguir kills fáceis
boost: um jogador sobe em cima do outro para alcançar posições elevadas
bunny/bunnar: pulos seguidos para se mover mais rápido
camper: jogador que fica parado em um local esperando inimigos passarem
capacete: acertar o capacete do inimigo sem matar — ele fica com vida baixíssima
carregar: levar o time nas costas; quem carrega é o melhor da equipe
cavalar: fazer muito barulho, geralmente correndo
clutch: situação crítica em que um jogador sozinho tenta ganhar a rodada
cone: jogador ruim, derruba fácil como um cone de trânsito
costinha: atacar pelas costas do inimigo sem que ele perceba
cravar a mira: deixar a mira estática esperando um inimigo passar
defusar: desarmar a bomba C4
drop/dropar: dar uma arma para um aliado sem dinheiro
eco: rodada em que o time economiza, sem comprar equipamento pesado
entry frag: primeira kill da entrada em um bombsite
fake: fintar, chamando atenção de um bombsite para depois atacar outro
flashar/bangar: jogar uma flashbang para cegar inimigos
flick: mover a mira rapidamente para acertar um tiro de sniper
forçar/force buy: comprar mesmo sem dinheiro suficiente
full control: ter sangue frio e controle total de uma situação
GG: "Good Game", bom jogo — dito ao final da partida
glockado: morrer para a pistola Glock, geralmente em eco adversário
halfado: estar com a vida pela metade (~50 HP)
hold/holdar: segurar posição e não sair dela
IGL: "In-Game Leader", o capitão que dita as estratégias
lurker: jogador que age separado do time para pegar informações ou flanquear
mel: tiro na cabeça que não mata, mas deixa o inimigo com pouquíssima vida
miado: estar com a vida muito baixa
mocado/marotado: escondido e pronto para pegar de surpresa
molotovar: jogar uma molotov em uma área
nade: granada (qualquer tipo)
nerf/nerfado: algo que foi piorado pela Valve
ninja defuse: desarmar a bomba sem os inimigos perceberem
noob: jogador iniciante ou ruim
one tap: matar com um único tiro, geralmente de AK-47 ou Desert Eagle
operar: matar com a faca
pé/pezinho: ajudar um aliado a subir em algum lugar dando impulso
peek: verificar rapidamente um ângulo expondo parte do corpo
peeker's advantage: vantagem física do jogador que abre a mira primeiro
pistolete: rodada comprando só colete e pistola
pino/pinar: jogador que erra muito; atirar e errar várias vezes
pop flash/perfeitinha: flashbang lançada perfeitamente, difícil de desviar
pré-fire: atirar antes mesmo de ver o inimigo, baseado em informação
pronet: jogar bem mas só na internet (nunca em LAN/presencial)
qué ota: expressão criada pelo BR Lucas1 ao acertar uma bala difícil na cabeça (especialmente de Deagle)
repick: sair de um pixel, mas voltar logo em seguida
rushar: invadir um local rapidamente sem tática
single: tiro único e fatal na cabeça
smurf: conta secundária de um jogador experiente em rank mais baixo
spray: atirar sem parar com uma arma automática, controlando o mouse
stack: colocar todos os jogadores no mesmo bombsite
strafe: encostar na quina e rápido, para pegar info sem se expor muito
taxado: alguém que foi cobrado ou responsabilizado por uma atitude ruim
tiltado: jogador afetado por estresse, rendendo abaixo do normal
TK/teamkill: acertar um aliado por acidente (ou propositalmente)
trade: vingar a morte de um aliado imediatamente
walk: andar segurando Shift para não fazer barulho
xiu: pedido de silêncio ("xiu, quieto!")
xitado: alguém usando cheats/hacks
zagueiro: jogador excessivamente passivo que deixa o time fazer tudo

=== FUNÇÕES DE JOGADOR ===
AWPer: especialista em sniper (AWP), responsável por controlar ângulos longos
anchor: primeiro a enfrentar o ataque no bombsite, segura sozinho
entry fragger: abre o caminho, faz o primeiro confronto
IGL: líder em jogo, comanda estratégias
lurker: age separado, busca informações e flancos
rifler: especialista em rifles (AK-47, M4)
support: ajuda com granadas, fumaças e flashbangs para a equipe

=== ECONOMIA ===
eco/eco round: rodada de economia sem comprar nada pesado
force buy: comprar mesmo sem dinheiro suficiente
full buy: rodada com compra completa — rifle, granadas e colete
pistolete: comprar só colete e pistola
save: guardar a arma em vez de arriscar morrer com ela

=== COMUNICAÇÃO TÁTICA ===
backup/back: pedido de apoio em um local
call: comunicar algo ao time (posição inimiga, jogada)
cover: proteção/cobertura para um aliado fazer algo
default: posicionamento padrão para coletar informações
exec: executar uma tática com granadas combinadas
rotate: reposicionar para defender ou atacar outro local
rush: atacar rapidamente um local sem estrutura de granadas
split: atacar de duas direções diferentes para o mesmo local
stack: concentrar jogadores em um lado

=== MECÂNICAS ===
ADR: "Average Damage per Round" — dano médio por rodada
bomb: a C4 que os TRs carregam e plantam no bombsite
burst fire: disparo em rajada de 3 balas (Glock, Famas)
FPS: frames por segundo — performance do PC no jogo
headshot/HS: tiro na cabeça
HP: pontos de vida do personagem
jump throw: pular e jogar granada simultaneamente (geralmente via bind)
lit: inimigo com vida baixa por dano recebido ("lit 60 no ramp")
molly: molotov ou incendiária
recoil: recuo da arma ao atirar — controlar o recoil é essencial
scope: mira telescópica (especialmente da AWP)
smoke/smokar: granada de fumaça — bloqueia linhas de visão
utility: conjunto de granadas (flash, smoke, molotov, HE)
varado: atirar através de paredes ou obstáculos

=== CALLOUTS POR MAPA ===
--- DUST 2 ---
long A: corredor longo que leva ao bombsite A
catwalk: passarela elevada do mid para o A
pit: posição baixa no bombsite A
goose: posição elevada no canto do bombsite A
short/ct short: passagem curta do mid para o A, pelo lado CT
xbox: caixa grande no meio do mapa (mid)
suicide: passagem estreita do spawn T para o mid
tunnels/lower tunnels: túneis subterrâneos que levam ao B
upper tunnels: entrada elevada dos túneis para o B
window: janela do mid para o B
car: veículo no bombsite B

--- MIRAGE ---
ramp: entrada principal T para o bombsite A
palace: entrada elevada T para o bombsite A
jungle: conector do mid para o A
stairs: escada CT para o A
firebox: posição de canto no bombsite A
triple/triple stack: três caixas sobrepostas no A
ninja (mirage): posição escondida embaixo do balcão do Palace
apartments/apps: apartamentos — entrada T para o B
kitchen: cômodo dentro dos apartments
market/market window: entrada CT para o B, pela janela
van: van estacionada no B
top mid: entrada T para o mid
window (mirage): janela do mid
connector: corredor conectando mid ao A
ladder room: sala com escada no mid

--- INFERNO ---
banana: corredor principal T para o bombsite B
sandbags: sacos de areia na banana
coffins: caixões no bombsite B
spools: carretéis de fio no B
arch: arco no bombsite A
pit (inferno): posição baixa no A
apartments/apps (inferno): entrada T para o A
balcony: posição elevada nos apps
boiler: sala da caldeira no mid
library: sala próxima ao A

--- NUKE ---
ramp: rampa de entrada para o site A (outside)
garage: garagem de entrada T
heaven (nuke): posição elevada sobre o site A
secret: passagem secreta conectando outside ao B
silo: silo externo do mapa
squeaky: porta barulhenta entre os sites
lobby: área de entrada para os CTs

--- OVERPASS ---
monster: túnel de entrada para o B
short (overpass): entrada curta para o B
heaven (overpass): posição elevada CT no B
water: área de água debaixo do B
bank: prédio perto do A
toilets/bathrooms: banheiros no meio do mapa
truck: caminhão no bombsite A
graffiti: muro com grafite no A

--- VERTIGO ---
ramp (vertigo): rampa de acesso principal
balconies: sacadas nas bordas do mapa
scaffolding: andaimes — posição elevada no B
ct mid: área central do CT
t mid: área central do T

--- ANCIENT ---
cave: caverna de entrada para o B
temple: templo central — área de conteste
tree: posição da árvore no A
donut: área circular no meio do templo

--- ANUBIS ---
canal: canal de entrada T
palace (anubis): estrutura de pedra no A
water (anubis): passagem por água para o B
bridge: ponte sobre o canal

=== ABREVIAÇÕES COMUNS ===
AK: AK-47 — rifle principal dos terroristas
AWP: sniper mais poderosa do jogo (Arctic Warfare Police)
CT: contra-terrorista
Deagle/DEagle: Desert Eagle — pistola poderosa
GG: good game (fim de partida)
GH: good half (fim do primeiro tempo)
GL: good luck (boa sorte, dito antes de partidas)
HE: granada explosiva
HF: have fun (divirta-se)
HP: pontos de vida
HS: headshot
M4: M4A4 ou M4A1-S — rifles dos CTs
MVP: melhor jogador da rodada
NT: nice try (boa tentativa)
TR: terrorista
VAC: Valve Anti-Cheat — sistema anti-cheats
`
}

// normalize converte um termo para lowercase e remove espaços extras.
func normalize(s string) string {
	result := ""
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			result += string(c + 32)
		} else {
			result += string(c)
		}
	}
	return result
}

// load popula o mapa com todos os termos.
func (l *CS2Lexicon) load() {
	add := func(t CS2Term) {
		l.terms[normalize(t.Term)] = t
	}

	// =====================
	// GÍRIAS E COMUNICAÇÃO
	// =====================
	add(CS2Term{
		Term:     "ace",
		Meaning:  "Matar todos os 5 inimigos na mesma rodada com um único jogador.",
		Category: "giria",
		Example:  "Cara, fiz um ace no pistol round!",
		Aliases:  []string{"neide", "neid"},
	})
	add(CS2Term{
		Term:     "neide",
		Meaning:  "Mesmo que ace — matar todos os 5 inimigos. Variação brasileira do termo.",
		Category: "giria",
		Example:  "Neide do cara foi incrível naquele clutch.",
		Aliases:  []string{"ace", "neid"},
	})
	add(CS2Term{
		Term:     "bait",
		Meaning:  "Usar um aliado como isca para conseguir kills mais fáceis e seguras.",
		Category: "giria",
		Example:  "Ele mandou o parceiro entrar na smoke e ficou baitando atrás.",
		Aliases:  []string{"baitar", "baiter"},
	})
	add(CS2Term{
		Term:     "boost",
		Meaning:  "Mecânica onde um jogador sobe em cima do outro para alcançar posições altas.",
		Category: "giria",
		Example:  "Faz boost em mim no box do B.",
		Aliases:  []string{"boostado"},
	})
	add(CS2Term{
		Term:     "bunny",
		Meaning:  "Pulos consecutivos sincronizados com o mouse para se mover mais rápido.",
		Category: "giria",
		Example:  "Ele bunnou pelo mid e entrou no A sem fazer barulho.",
		Aliases:  []string{"bunnar", "bunny jump"},
	})
	add(CS2Term{
		Term:     "camper",
		Meaning:  "Jogador que fica parado no mesmo local por longo tempo esperando inimigos.",
		Category: "giria",
		Example:  "Cuidado, tem um camper no corner do B.",
		Aliases:  []string{"campear"},
	})
	add(CS2Term{
		Term:     "capacete",
		Meaning:  "Tiro no capacete do inimigo que não o mata, deixando-o com vida baixíssima.",
		Category: "giria",
		Example:  "Dei capacete nele, alguém finaliza!",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "carregar",
		Meaning:  "Levar o time nas costas; ser o responsável pelas vitórias do time.",
		Category: "giria",
		Example:  "Tô carregando esse time faz 5 rounds.",
		Aliases:  []string{"carregado", "mochila"},
	})
	add(CS2Term{
		Term:     "cavalar",
		Meaning:  "Fazer muito barulho correndo ou pulando sem necessidade.",
		Category: "giria",
		Example:  "Para de cavalar, o inimigo vai ouvir você!",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "clutch",
		Meaning:  "Situação em que um jogador sozinho tenta ganhar a rodada em desvantagem numérica.",
		Category: "giria",
		Example:  "1v3 no clutch, vamos lá!",
		Aliases:  []string{"clutchar", "clutch master"},
	})
	add(CS2Term{
		Term:     "cone",
		Meaning:  "Jogador ruim que é facilmente eliminado, como um cone de trânsito.",
		Category: "giria",
		Example:  "Esse cara é um cone, morreu na primeira troca.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "costinha",
		Meaning:  "Atacar pelas costas do inimigo sem que ele perceba; flanquear.",
		Category: "giria",
		Example:  "Cuidado com costinha pelo CT!",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "cravar a mira",
		Meaning:  "Deixar a mira estática em um local esperando que um inimigo passe (pre-aim).",
		Category: "giria",
		Example:  "Cravei a mira na saída do tunnel e matei os dois.",
		Aliases:  []string{"pre-aim", "aim estático"},
	})
	add(CS2Term{
		Term:     "drop",
		Meaning:  "Dar uma arma para um aliado que está sem dinheiro para comprar.",
		Category: "giria",
		Example:  "Me dropa uma M4, to sem grana.",
		Aliases:  []string{"dropar"},
	})
	add(CS2Term{
		Term:     "eco",
		Meaning:  "Rodada em que o time economiza dinheiro comprando o mínimo possível.",
		Category: "economia",
		Example:  "Vai eco esse round, tamo sem grana.",
		Aliases:  []string{"eco round", "save"},
	})
	add(CS2Term{
		Term:     "fake",
		Meaning:  "Simular ataque em um bombsite para distrair os CTs e atacar o outro.",
		Category: "tatica",
		Example:  "Vai jogar fake no A, depois todo mundo entra no B.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "flashar",
		Meaning:  "Jogar uma flashbang para cegar inimigos.",
		Category: "giria",
		Example:  "Flasha o corner antes de entrar!",
		Aliases:  []string{"bangar", "flash"},
	})
	add(CS2Term{
		Term:     "flick",
		Meaning:  "Mover a mira de forma rápida e precisa para acertar um tiro, especialmente de AWP.",
		Category: "giria",
		Example:  "Que flick incrível naquele long A!",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "forçar",
		Meaning:  "Comprar equipamento mesmo com pouco dinheiro, arriscando a rodada.",
		Category: "economia",
		Example:  "Vamos forçar esse round, não temos escolha.",
		Aliases:  []string{"force buy", "forçar round"},
	})
	add(CS2Term{
		Term:     "full buy",
		Meaning:  "Rodada com compra completa: rifle principal, granadas e colete.",
		Category: "economia",
		Example:  "Esse é full buy, todo mundo pega rifle.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "GG",
		Meaning:  "'Good Game' — expressão de respeito ao final de uma partida.",
		Category: "comunicacao",
		Example:  "GG wp pessoal!",
		Aliases:  []string{"gg wp", "good game"},
	})
	add(CS2Term{
		Term:     "glockado",
		Meaning:  "Morrer para a pistola Glock, geralmente em uma rodada de eco.",
		Category: "giria",
		Example:  "Cara, fui glockado no pistol round!",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "halfado",
		Meaning:  "Estar com aproximadamente metade da vida (50 HP).",
		Category: "giria",
		Example:  "Tô halfado, vai com cuidado.",
		Aliases:  []string{"half hp"},
	})
	add(CS2Term{
		Term:     "hold",
		Meaning:  "Segurar uma posição sem recuar até segunda ordem.",
		Category: "tatica",
		Example:  "Holda o banana, não deixa eles entrar.",
		Aliases:  []string{"holdar"},
	})
	add(CS2Term{
		Term:     "lit",
		Meaning:  "Inimigo que recebeu dano e está com vida baixa.",
		Category: "comunicacao",
		Example:  "Lit 60 no ramp, fica esperto!",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "mel",
		Meaning:  "Headshot que não mata, mas deixa o inimigo com vida mínima (1-5 HP).",
		Category: "giria",
		Example:  "Dei mel nele — alguém finaliza!",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "miado",
		Meaning:  "Estar com a vida muito baixa, perto de morrer.",
		Category: "giria",
		Example:  "Tô miado com 8 de vida, preciso de healer... ops, errado jogo.",
		Aliases:  []string{"low hp", "baixo"},
	})
	add(CS2Term{
		Term:     "mocado",
		Meaning:  "Estar escondido e preparado para pegar um inimigo de surpresa.",
		Category: "giria",
		Example:  "Tô mocado no corner do A, vem um aqui.",
		Aliases:  []string{"marotado"},
	})
	add(CS2Term{
		Term:     "ninja defuse",
		Meaning:  "Desarmar a C4 de forma furtiva, sem os inimigos perceberem.",
		Category: "giria",
		Example:  "Fiz ninja defuse enquanto eles brigavam no B.",
		Aliases:  []string{"ninja"},
	})
	add(CS2Term{
		Term:     "one tap",
		Meaning:  "Matar com um único clique e um tiro perfeitamente colocado.",
		Category: "giria",
		Example:  "One tap de Deagle nele!",
		Aliases:  []string{"single"},
	})
	add(CS2Term{
		Term:     "operar",
		Meaning:  "Matar um inimigo com a faca.",
		Category: "giria",
		Example:  "Operei ele pelo CT, nem vi vir.",
		Aliases:  []string{"faca", "knifar"},
	})
	add(CS2Term{
		Term:     "peek",
		Meaning:  "Verificar um ângulo rapidamente, expondo parte do corpo para ver ou engajar.",
		Category: "giria",
		Example:  "Peeka aqui para mim ver se tem alguém.",
		Aliases:  []string{"peekar"},
	})
	add(CS2Term{
		Term:     "pistolete",
		Meaning:  "Rodada comprando apenas colete e pistola, sem rifle.",
		Category: "economia",
		Example:  "Vai de pistolete esse round.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "pino",
		Meaning:  "Jogador que erra muito; pinar é errar vários tiros seguidos.",
		Category: "giria",
		Example:  "Para de pinar com a AK, controla o spray!",
		Aliases:  []string{"pinar"},
	})
	add(CS2Term{
		Term:     "pop flash",
		Meaning:  "Flashbang lançada perfeitamente, difícil de evitar — 'estoura' na frente do inimigo.",
		Category: "giria",
		Example:  "Joga uma pop flash no site antes de entrar.",
		Aliases:  []string{"perfeitinha"},
	})
	add(CS2Term{
		Term:     "pré-fire",
		Meaning:  "Atirar antes de ver o inimigo, baseado em informação prévia sobre sua posição.",
		Category: "giria",
		Example:  "Pré-fire o corner do B, sempre tem alguém ali.",
		Aliases:  []string{"prefire"},
	})
	add(CS2Term{
		Term:     "qué ota",
		Meaning:  "Expressão criada pelo pro player brasileiro Lucas1; usada ao acertar uma bala difícil na cabeça.",
		Category: "giria",
		Example:  "Qué ota! Deagle no long A!",
		Aliases:  []string{"que ota"},
	})
	add(CS2Term{
		Term:     "repick",
		Meaning:  "Sair de um pixel e voltar imediatamente, enganando o inimigo.",
		Category: "giria",
		Example:  "Faz um repick rápido pra tirar a info.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "rushar",
		Meaning:  "Invadir rapidamente um local sem estrutura tática.",
		Category: "tatica",
		Example:  "Rush B, não para!",
		Aliases:  []string{"rush", "rushar"},
	})
	add(CS2Term{
		Term:     "save",
		Meaning:  "Guardar a arma ao invés de arriscar morrer com ela quando a rodada está perdida.",
		Category: "economia",
		Example:  "Sava a arma, a rodada tá perdida.",
		Aliases:  []string{"savar"},
	})
	add(CS2Term{
		Term:     "spray",
		Meaning:  "Atirar continuamente com uma arma automática controlando o recoil com o mouse.",
		Category: "giria",
		Example:  "Controla o spray da AK, não solta o mouse.",
		Aliases:  []string{"sprayan", "controle de spray"},
	})
	add(CS2Term{
		Term:     "stack",
		Meaning:  "Concentrar vários jogadores no mesmo bombsite.",
		Category: "tatica",
		Example:  "Stack B, eles tão indo tudo pro B.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "strafe",
		Meaning:  "Encostar rapidamente na quina de uma parede para obter informação ou provocar erros.",
		Category: "giria",
		Example:  "Faz strafe no box antes de entrar.",
		Aliases:  []string{"strafar"},
	})
	add(CS2Term{
		Term:     "tiltado",
		Meaning:  "Jogador emocionalmente afetado pelo jogo, rendendo abaixo do esperado.",
		Category: "giria",
		Example:  "Ele tá tiltado depois daquele clutch perdido.",
		Aliases:  []string{"tilt"},
	})
	add(CS2Term{
		Term:     "trade",
		Meaning:  "Vingar a morte de um aliado imediatamente após ele cair.",
		Category: "tatica",
		Example:  "Tradeou o kill, 1 por 1.",
		Aliases:  []string{"tradear"},
	})
	add(CS2Term{
		Term:     "walk",
		Meaning:  "Andar segurando Shift para não fazer barulho e passar despercebido.",
		Category: "giria",
		Example:  "Entra de walk pelo CT, sem correr.",
		Aliases:  []string{"walkear"},
	})
	add(CS2Term{
		Term:     "xiu",
		Meaning:  "Pedido de silêncio — parar de fazer barulho no mapa.",
		Category: "comunicacao",
		Example:  "Xiu, xiu! Ouvi passos no ramp.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "xitado",
		Meaning:  "Jogador usando cheats ou hacks para ter vantagem ilegal.",
		Category: "giria",
		Example:  "Esse cara é xitado, headshot em todo round.",
		Aliases:  []string{"cheater", "hackudo"},
	})
	add(CS2Term{
		Term:     "zagueiro",
		Meaning:  "Jogador excessivamente defensivo que não contribui com o time.",
		Category: "giria",
		Example:  "Para de ser zagueiro, ajuda o time no B.",
		Aliases:  []string{},
	})

	// =====================
	// FUNÇÕES DE JOGADOR
	// =====================
	add(CS2Term{
		Term:     "IGL",
		Meaning:  "In-Game Leader — capitão do time que comanda estratégias e faz calls durante a partida.",
		Category: "funcao",
		Example:  "O IGL mandou todo mundo ir pro B.",
		Aliases:  []string{"in-game leader", "capitão"},
	})
	add(CS2Term{
		Term:     "entry fragger",
		Meaning:  "Função: ser a linha de frente, fazer a primeira entrada e o primeiro confronto do time.",
		Category: "funcao",
		Example:  "O entry fragger entrou primeiro e fez a limpa.",
		Aliases:  []string{"entry"},
	})
	add(CS2Term{
		Term:     "AWPer",
		Meaning:  "Especialista em AWP — controla ângulos longos, crucial em confrontos de média/longa distância.",
		Category: "funcao",
		Example:  "O AWPer tá dominando o mid inteiro.",
		Aliases:  []string{"sniper"},
	})
	add(CS2Term{
		Term:     "lurker",
		Meaning:  "Função: age separado do time, busca informações e oportunidades de flanquear.",
		Category: "funcao",
		Example:  "O lurker pegou dois pelo CT enquanto entrávamos no B.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "support",
		Meaning:  "Função: especialista em granadas — lança flashes, smokes e molotovs para facilitar a entrada do time.",
		Category: "funcao",
		Example:  "Support vai lançar as smokes do CT antes de entrar.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "anchor",
		Meaning:  "Função: primeiro a enfrentar o ataque inimigo no bombsite, segura sozinho pelo máximo de tempo.",
		Category: "funcao",
		Example:  "Você vai ser o anchor do B, segura até o rotate chegar.",
		Aliases:  []string{},
	})

	// =====================
	// COMUNICAÇÃO TÁTICA
	// =====================
	add(CS2Term{
		Term:     "call",
		Meaning:  "Comunicar algo ao time: posição inimiga, jogada planejada ou alerta.",
		Category: "comunicacao",
		Example:  "Faz o call antes de entrar!",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "rotate",
		Meaning:  "Reposicionar para defender ou atacar outro local do mapa.",
		Category: "tatica",
		Example:  "Dois rotacionam pro A agora!",
		Aliases:  []string{"rotacionar"},
	})
	add(CS2Term{
		Term:     "default",
		Meaning:  "Posicionamento padrão para coletar informações antes de decidir a jogada.",
		Category: "tatica",
		Example:  "Vai de default, pega a info primeiro.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "exec",
		Meaning:  "Executar uma tática combinada com uso sincronizado de granadas.",
		Category: "tatica",
		Example:  "Exec B agora: smoke CT, flash ramp, molly banana.",
		Aliases:  []string{"execução"},
	})
	add(CS2Term{
		Term:     "split",
		Meaning:  "Atacar o mesmo bombsite por duas direções diferentes simultaneamente.",
		Category: "tatica",
		Example:  "Split no A: três pelo ramp, dois pelo short.",
		Aliases:  []string{},
	})

	// =====================
	// POSIÇÕES — DUST 2
	// =====================
	add(CS2Term{
		Term:     "long A",
		Meaning:  "[Dust 2] Corredor longo que leva ao bombsite A — posição crítica da equipe T.",
		Category: "posicao",
		Example:  "Dois no long A, um no short.",
		Aliases:  []string{"long", "long doors"},
	})
	add(CS2Term{
		Term:     "catwalk",
		Meaning:  "[Dust 2] Passarela elevada conectando o mid ao bombsite A.",
		Category: "posicao",
		Example:  "Alguém subiu catwalk, cuidado no A.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "pit",
		Meaning:  "[Dust 2] Posição baixa no bombsite A — excelente para AWP ou hold defensivo.",
		Category: "posicao",
		Example:  "Tem alguém no pit — flasha antes de entrar.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "goose",
		Meaning:  "[Dust 2] Posição elevada no canto do bombsite A, perto do plant.",
		Category: "posicao",
		Example:  "CT no goose, cuidado!",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "short",
		Meaning:  "[Dust 2] Passagem curta do mid para o A — lado CT.",
		Category: "posicao",
		Example:  "Abre o short antes de entrar.",
		Aliases:  []string{"ct short"},
	})
	add(CS2Term{
		Term:     "xbox",
		Meaning:  "[Dust 2] Caixa grande no meio do mapa (mid) — cobertura comum.",
		Category: "posicao",
		Example:  "Passou pelo xbox no mid.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "tunnels",
		Meaning:  "[Dust 2] Túneis subterrâneos que levam ao bombsite B.",
		Category: "posicao",
		Example:  "Dois entrando pelos tunnels.",
		Aliases:  []string{"lower tunnels", "upper tunnels"},
	})

	// =====================
	// POSIÇÕES — MIRAGE
	// =====================
	add(CS2Term{
		Term:     "ramp",
		Meaning:  "[Mirage] Entrada principal T para o bombsite A.",
		Category: "posicao",
		Example:  "Dois no ramp, dois no palace.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "palace",
		Meaning:  "[Mirage] Entrada elevada T para o bombsite A.",
		Category: "posicao",
		Example:  "Jogadores entrando pelo palace.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "apartments",
		Meaning:  "[Mirage / Inferno] Apartamentos — entrada T para o B (Mirage) ou A (Inferno).",
		Category: "posicao",
		Example:  "Todos nos apps, entramos pelo B.",
		Aliases:  []string{"apps"},
	})
	add(CS2Term{
		Term:     "kitchen",
		Meaning:  "[Mirage] Cômodo dentro dos apartments, caminho para o B.",
		Category: "posicao",
		Example:  "Alguém na kitchen esperando.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "connector",
		Meaning:  "[Mirage] Corredor que conecta o mid ao bombsite A.",
		Category: "posicao",
		Example:  "Controla o connector antes de empurrar.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "window",
		Meaning:  "[Mirage] Janela do mid — posição elevada e de controle central.",
		Category: "posicao",
		Example:  "AWP na window, não passa ninguém.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "van",
		Meaning:  "[Mirage] Van estacionada no bombsite B — cobertura popular.",
		Category: "posicao",
		Example:  "Tem CT atrás da van.",
		Aliases:  []string{},
	})

	// =====================
	// POSIÇÕES — INFERNO
	// =====================
	add(CS2Term{
		Term:     "banana",
		Meaning:  "[Inferno] Corredor principal T para o bombsite B — congestionado e perigoso.",
		Category: "posicao",
		Example:  "Molly o banana antes de empurrar!",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "coffins",
		Meaning:  "[Inferno] Caixões no bombsite B — cobertura comum para plant e hold.",
		Category: "posicao",
		Example:  "Planta atrás dos coffins.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "spools",
		Meaning:  "[Inferno] Carretéis de fio no bombsite B — posição para cover.",
		Category: "posicao",
		Example:  "CT nos spools, flasha antes.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "arch",
		Meaning:  "[Inferno] Arco de entrada CT no bombsite A.",
		Category: "posicao",
		Example:  "Smoke o arch no A exec.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "boiler",
		Meaning:  "[Inferno] Sala da caldeira — área de mid conectando para o A.",
		Category: "posicao",
		Example:  "Controla o boiler para chegar no A.",
		Aliases:  []string{},
	})

	// =====================
	// POSIÇÕES — NUKE
	// =====================
	add(CS2Term{
		Term:     "heaven",
		Meaning:  "[Nuke / Overpass] Posição elevada com visão privilegiada sobre o site.",
		Category: "posicao",
		Example:  "CT no heaven, fica esperto.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "secret",
		Meaning:  "[Nuke] Passagem secreta conectando o exterior ao bombsite B — rotação alternativa.",
		Category: "posicao",
		Example:  "Dois indo pelo secret para o B.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "squeaky",
		Meaning:  "[Nuke] Porta barulhenta entre os sites A e B — abre-la dá informação ao inimigo.",
		Category: "posicao",
		Example:  "Alguém abriu o squeaky, vai B!",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "silo",
		Meaning:  "[Nuke] Estrutura de silo no exterior do mapa.",
		Category: "posicao",
		Example:  "Pegou a posição no silo.",
		Aliases:  []string{},
	})

	// =====================
	// POSIÇÕES — OVERPASS
	// =====================
	add(CS2Term{
		Term:     "monster",
		Meaning:  "[Overpass] Túnel de entrada dos T para o bombsite B.",
		Category: "posicao",
		Example:  "Rush pelo monster, não para!",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "bank",
		Meaning:  "[Overpass] Prédio perto do bombsite A — posição de controle.",
		Category: "posicao",
		Example:  "CT no bank, segurando o A.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "water",
		Meaning:  "[Overpass] Área de água abaixo do bombsite B — passagem alternativa.",
		Category: "posicao",
		Example:  "Entrando pelo water por baixo.",
		Aliases:  []string{},
	})

	// =====================
	// ARMAS
	// =====================
	add(CS2Term{
		Term:     "AK",
		Meaning:  "AK-47 — rifle principal dos terroristas, alta potência, mata com headshot em qualquer distância.",
		Category: "arma",
		Example:  "Pega a AK esse round.",
		Aliases:  []string{"AK-47"},
	})
	add(CS2Term{
		Term:     "AWP",
		Meaning:  "Arctic Warfare Police — sniper mais poderosa do jogo. Mata com um tiro no corpo. Cara ($4750).",
		Category: "arma",
		Example:  "O AWPer travou o mid inteiro.",
		Aliases:  []string{"sniper", "awp"},
	})
	add(CS2Term{
		Term:     "M4",
		Meaning:  "M4A4 ou M4A1-S — rifles principais dos contra-terroristas.",
		Category: "arma",
		Example:  "Pega a M4A1-S que faz menos barulho.",
		Aliases:  []string{"M4A4", "M4A1-S"},
	})
	add(CS2Term{
		Term:     "Deagle",
		Meaning:  "Desert Eagle — pistola poderosa, mata com headshot de qualquer distância.",
		Category: "arma",
		Example:  "Qué ota, Deagle no long!",
		Aliases:  []string{"DEagle"},
	})
	add(CS2Term{
		Term:     "molly",
		Meaning:  "Molotov ou granada incendiária — cria uma área de fogo bloqueando passagens.",
		Category: "arma",
		Example:  "Joga a molly no banana agora.",
		Aliases:  []string{"molotov", "inc", "incendiária"},
	})
	add(CS2Term{
		Term:     "smoke",
		Meaning:  "Granada de fumaça — bloqueia linhas de visão por 18 segundos.",
		Category: "arma",
		Example:  "Smoke o CT antes de entrar.",
		Aliases:  []string{"smokar", "fumaça"},
	})
	add(CS2Term{
		Term:     "flash",
		Meaning:  "Flashbang — cega e ensurdece inimigos temporariamente.",
		Category: "arma",
		Example:  "Joga uma flash antes de entrar no corner.",
		Aliases:  []string{"flashbang", "flashar"},
	})
	add(CS2Term{
		Term:     "HE",
		Meaning:  "Granada explosiva — causa dano em área.",
		Category: "arma",
		Example:  "Jogou HE no meio deles.",
		Aliases:  []string{"HE nade", "nade"},
	})

	// =====================
	// MECÂNICAS E ESTATÍSTICAS
	// =====================
	add(CS2Term{
		Term:     "ADR",
		Meaning:  "Average Damage per Round — dano médio por rodada. Métrica de desempenho do jogador.",
		Category: "giria",
		Example:  "Meu ADR tá em 95 essa partida.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "recoil",
		Meaning:  "Recuo da arma ao atirar — controlar o recoil (spray control) é habilidade essencial.",
		Category: "giria",
		Example:  "Controla o recoil da AK, ela puxa pra cima.",
		Aliases:  []string{"recuo", "spray control"},
	})
	add(CS2Term{
		Term:     "peeker's advantage",
		Meaning:  "Vantagem física do jogador que abre a mira versus o que está parado esperando — latência favorece quem move.",
		Category: "giria",
		Example:  "Perdeu por causa do peeker's advantage, estava muito estático.",
		Aliases:  []string{},
	})
	add(CS2Term{
		Term:     "varado",
		Meaning:  "Atirar através de paredes ou obstáculos causando dano (wallbang).",
		Category: "giria",
		Example:  "Varei ele pela parede do mid.",
		Aliases:  []string{"wallbang", "varar"},
	})
}
