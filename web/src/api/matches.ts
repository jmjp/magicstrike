import { api } from "./client";
import type { ApiResponse, Match, PaginatedMatches } from "./types";

/**
 * GET /matches — List user's matches (paginated).
 */
export async function listMatches(
  limit = 20,
  offset = 0,
): Promise<PaginatedMatches> {
  const { data } = await api.get<ApiResponse<PaginatedMatches>>("/matches", {
    params: { limit, offset },
  });
  return data.data;
}

/**
 * GET /matches/:id — Get single match details.
 */
export async function getMatch(id: string): Promise<Match> {
  const { data } = await api.get<ApiResponse<Match>>(`/matches/${id}`);
  return data.data;
}
