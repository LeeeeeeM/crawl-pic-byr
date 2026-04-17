import type { BYRCrawlConfig, BaiduIndexCrawlConfig, CDPCrawlConfig, CrawlConfig, CrawlJob, Photo, Post } from './types';

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '';
export const API_BASE_URL = BASE_URL;

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${BASE_URL}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
  });

  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || `HTTP ${response.status}`);
  }

  return response.json() as Promise<T>;
}

export function createJob(config: CrawlConfig): Promise<CrawlJob> {
  return request<CrawlJob>('/api/jobs', {
    method: 'POST',
    body: JSON.stringify(config),
  });
}

export function createBYRJob(config: BYRCrawlConfig): Promise<CrawlJob> {
  return request<CrawlJob>('/api/byr/jobs', {
    method: 'POST',
    body: JSON.stringify(config),
  });
}

export function createCDPJob(config: CDPCrawlConfig): Promise<CrawlJob> {
  return request<CrawlJob>('/api/cdp/jobs', {
    method: 'POST',
    body: JSON.stringify(config),
  });
}

export function createBaiduIndexJob(config: BaiduIndexCrawlConfig): Promise<CrawlJob> {
  return request<CrawlJob>('/api/baidu-index/jobs', {
    method: 'POST',
    body: JSON.stringify(config),
  });
}

export function getJob(id: string): Promise<CrawlJob> {
  return request<CrawlJob>(`/api/jobs/${id}`);
}

export function listJobs(limit = 50): Promise<CrawlJob[]> {
  return request<CrawlJob[]>(`/api/jobs?limit=${limit}`);
}

export function cancelJob(id: string): Promise<{ id: string; status: string }> {
  return request<{ id: string; status: string }>(`/api/jobs/${id}/cancel`, {
    method: 'POST',
  });
}

export function deleteJob(id: string): Promise<{ id: string; status: string }> {
  return request<{ id: string; status: string }>(`/api/jobs/${id}`, {
    method: 'DELETE',
  });
}

export function getPosts(id: string): Promise<Post[]> {
  return request<Post[]>(`/api/jobs/${id}/posts`);
}

export function getPhotos(id: string): Promise<Photo[]> {
  return request<Photo[]>(`/api/jobs/${id}/photos`);
}
