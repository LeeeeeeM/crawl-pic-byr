export type CrawlConfig = {
  siteName: string;
  startUrls: string[];
  allowedDomains: string[];
  postLinkSelector: string;
  nextPageSelector: string;
  imageSelector: string;
  postTitleSelector: string;
  maxListPages: number;
  maxPosts: number;
  requestTimeoutSecs: number;
  minImageBytes: number;
};

export type BYRCrawlConfig = {
  siteName: string;
  boardName: string;
  startPage: number;
  maxPages: number;
  remoteDebugUrl: string;
  minImageBytes: number;
};

export type CDPCrawlConfig = {
  siteName: string;
  startUrl: string;
  remoteDebugUrl: string;
  pageReadySelector: string;
  contentSelector: string;
  imageSelector: string;
  titleSelector: string;
  waitAfterLoadMs: number;
  minImageBytes: number;
};

export type BaiduIndexCrawlConfig = {
  siteName: string;
  keyword: string;
  startUrl: string;
  period: '7d' | '30d' | '90d' | '180d' | 'all';
  remoteDebugUrl: string;
  waitAfterLoadMs: number;
  minImageBytes: number;
};

export type CrawlJob = {
  id: string;
  siteName: string;
  status: 'pending' | 'running' | 'done' | 'failed';
  error: string | null;
  startedAt: string | null;
  finishedAt: string | null;
  createdAt: string;
};

export type Post = {
  id: number;
  jobId: string;
  title: string;
  content: string;
  url: string;
  createdAt: string;
};

export type Photo = {
  id: number;
  jobId: string;
  postId: number;
  url: string;
  fileName: string | null;
  altText: string | null;
  createdAt: string;
};
