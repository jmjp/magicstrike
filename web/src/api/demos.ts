import axios from "axios";
import { api } from "./client";
import type { UploadRequestData } from "./types";

/**
 * POST /demos/upload-request — Get a pre-signed S3 URL for demo upload.
 */
export async function requestUpload(params: {
  match_id?: string;
  filename: string;
  md5_hash?: string;
  team_a: string;
  team_b: string;
  map_name: string;
}): Promise<UploadRequestData> {
  const { data } = await api.post<UploadRequestData>(
    "/demos/upload-request",
    params,
  );
  return data;
}

/**
 * PUT file directly to the pre-signed S3 URL.
 */
export async function uploadToS3(
  url: string,
  file: File,
  onProgress?: (pct: number) => void,
): Promise<void> {
  await axios.put(url, file, {
    headers: { "Content-Type": "application/octet-stream" },
    onUploadProgress: (e) => {
      if (e.total && onProgress) {
        onProgress(Math.round((e.loaded * 100) / e.total));
      }
    },
  });
}

/**
 * POST /demos/upload-confirm — Confirm upload and enqueue processing.
 */
export async function confirmUpload(
  match_id: string,
  bucket_key: string,
): Promise<{ status: string; match_id: string }> {
  const { data } = await api.post<{
    status: string;
    match_id: string;
  }>("/demos/upload-confirm", { match_id, bucket_key });
  return data;
}
