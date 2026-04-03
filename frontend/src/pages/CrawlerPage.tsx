import { FormEvent, useMemo, useState } from 'react';
import { createBYRJob, createJob, getJob, getPhotos, getPosts } from '../api';
import type { BYRCrawlConfig, CrawlConfig, CrawlJob, Photo, Post } from '../types';

const defaultGenericForm = {
  siteName: '',
  startUrls: '',
  allowedDomains: '',
  postLinkSelector: 'a.post-link',
  nextPageSelector: 'a.next',
  imageSelector: 'article img',
  postTitleSelector: 'h1',
  maxListPages: 10,
  maxPosts: 200,
  requestTimeoutSecs: 20,
  minImageBytes: 51200,
};

const defaultBYRForm = {
  siteName: 'byr-friends',
  boardName: 'Friends',
  startPage: 1,
  maxPages: 2000,
  remoteDebugUrl: 'http://127.0.0.1:9222',
  minImageBytes: 51200,
};

function splitLines(raw: string): string[] {
  return raw
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);
}

export default function App() {
  const [tab, setTab] = useState<'byr' | 'generic'>('byr');
  const [genericForm, setGenericForm] = useState(defaultGenericForm);
  const [byrForm, setByrForm] = useState(defaultBYRForm);
  const [job, setJob] = useState<CrawlJob | null>(null);
  const [posts, setPosts] = useState<Post[]>([]);
  const [photos, setPhotos] = useState<Photo[]>([]);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const photosByPost = useMemo(() => {
    const map = new Map<number, Photo[]>();
    for (const photo of photos) {
      const list = map.get(photo.postId) ?? [];
      list.push(photo);
      map.set(photo.postId, list);
    }
    return map;
  }, [photos]);

  const stats = useMemo(
    () => ({
      postCount: posts.length,
      photoCount: photos.length,
    }),
    [posts.length, photos.length],
  );

  async function onGenericSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError('');
    setSubmitting(true);
    setPosts([]);
    setPhotos([]);

    const payload: CrawlConfig = {
      siteName: genericForm.siteName.trim(),
      startUrls: splitLines(genericForm.startUrls),
      allowedDomains: splitLines(genericForm.allowedDomains),
      postLinkSelector: genericForm.postLinkSelector.trim(),
      nextPageSelector: genericForm.nextPageSelector.trim(),
      imageSelector: genericForm.imageSelector.trim(),
      postTitleSelector: genericForm.postTitleSelector.trim(),
      maxListPages: Number(genericForm.maxListPages),
      maxPosts: Number(genericForm.maxPosts),
      requestTimeoutSecs: Number(genericForm.requestTimeoutSecs),
      minImageBytes: Number(genericForm.minImageBytes),
    };

    try {
      const created = await createJob(payload);
      setJob(created);
      await pollJob(created.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建任务失败');
    } finally {
      setSubmitting(false);
    }
  }

  async function onBYRSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError('');
    setSubmitting(true);
    setPosts([]);
    setPhotos([]);

    const payload: BYRCrawlConfig = {
      siteName: byrForm.siteName.trim(),
      boardName: byrForm.boardName.trim(),
      startPage: Number(byrForm.startPage),
      maxPages: Number(byrForm.maxPages),
      remoteDebugUrl: byrForm.remoteDebugUrl.trim(),
      minImageBytes: Number(byrForm.minImageBytes),
    };

    try {
      const created = await createBYRJob(payload);
      setJob(created);
      await pollJob(created.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建 BYR 任务失败');
    } finally {
      setSubmitting(false);
    }
  }

  async function pollJob(jobId: string): Promise<void> {
    for (let i = 0; i < 600; i += 1) {
      const latest = await getJob(jobId);
      setJob(latest);

      if (latest.status === 'done') {
        const [loadedPosts, loadedPhotos] = await Promise.all([getPosts(jobId), getPhotos(jobId)]);
        setPosts(loadedPosts);
        setPhotos(loadedPhotos);
        return;
      }

      if (latest.status === 'failed') {
        setError(latest.error ?? '任务失败');
        return;
      }

      await new Promise((resolve) => setTimeout(resolve, 2000));
    }

    setError('轮询超时，请稍后手动刷新任务状态');
  }

  return (
    <div className="container">
      <h1>帖子爬虫</h1>
      <p className="subtitle">抓取帖子内容和图片 URL（无图片帖子不入库）</p>

      <section className="card tabs">
        <button type="button" className={tab === 'byr' ? 'active' : ''} onClick={() => setTab('byr')}>
          BYR 登录态爬取
        </button>
        <button type="button" className={tab === 'generic' ? 'active' : ''} onClick={() => setTab('generic')}>
          通用选择器爬取
        </button>
      </section>

      {tab === 'byr' && (
        <form className="card form-grid" onSubmit={onBYRSubmit}>
          <p className="muted">
            先启动 Chrome 调试端口并手动登录 BYR，再提交任务。默认命令：
            <code> /Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --remote-debugging-port=9222</code>
          </p>
          <p className="muted">图片过滤默认阈值为 50KB（51200 字节）。</p>
          <label>
            任务名
            <input
              required
              value={byrForm.siteName}
              onChange={(e) => setByrForm((prev) => ({ ...prev, siteName: e.target.value }))}
            />
          </label>
          <label>
            板块名
            <input
              required
              value={byrForm.boardName}
              onChange={(e) => setByrForm((prev) => ({ ...prev, boardName: e.target.value }))}
            />
          </label>
          <div className="inline-inputs">
            <label>
              起始页
              <input
                type="number"
                min={1}
                value={byrForm.startPage}
                onChange={(e) => setByrForm((prev) => ({ ...prev, startPage: Number(e.target.value) }))}
              />
            </label>
            <label>
              最大页数
              <input
                type="number"
                min={1}
                value={byrForm.maxPages}
                onChange={(e) => setByrForm((prev) => ({ ...prev, maxPages: Number(e.target.value) }))}
              />
            </label>
            <label>
              最小图片大小(字节)
              <input
                type="number"
                min={0}
                value={byrForm.minImageBytes}
                onChange={(e) => setByrForm((prev) => ({ ...prev, minImageBytes: Number(e.target.value) }))}
              />
            </label>
          </div>
          <label>
            Chrome DevTools 地址
            <input
              required
              value={byrForm.remoteDebugUrl}
              onChange={(e) => setByrForm((prev) => ({ ...prev, remoteDebugUrl: e.target.value }))}
            />
          </label>
          <button type="submit" disabled={submitting}>
            {submitting ? '采集中...' : '开始 BYR 爬取'}
          </button>
        </form>
      )}

      {tab === 'generic' && (
        <form className="card form-grid" onSubmit={onGenericSubmit}>
          <p className="muted">图片过滤默认阈值为 50KB（51200 字节）。</p>
          <label>
            站点名称
            <input
              required
              value={genericForm.siteName}
              onChange={(e) => setGenericForm((prev) => ({ ...prev, siteName: e.target.value }))}
            />
          </label>

          <label>
            起始列表页（每行一个 URL）
            <textarea
              required
              rows={3}
              value={genericForm.startUrls}
              onChange={(e) => setGenericForm((prev) => ({ ...prev, startUrls: e.target.value }))}
            />
          </label>

          <label>
            允许域名（每行一个，如 example.com）
            <textarea
              rows={2}
              value={genericForm.allowedDomains}
              onChange={(e) => setGenericForm((prev) => ({ ...prev, allowedDomains: e.target.value }))}
            />
          </label>

          <label>
            帖子链接选择器
            <input
              required
              value={genericForm.postLinkSelector}
              onChange={(e) => setGenericForm((prev) => ({ ...prev, postLinkSelector: e.target.value }))}
            />
          </label>

          <label>
            下一页选择器（可空）
            <input
              value={genericForm.nextPageSelector}
              onChange={(e) => setGenericForm((prev) => ({ ...prev, nextPageSelector: e.target.value }))}
            />
          </label>

          <label>
            图片选择器
            <input
              value={genericForm.imageSelector}
              onChange={(e) => setGenericForm((prev) => ({ ...prev, imageSelector: e.target.value }))}
            />
          </label>

          <label>
            标题选择器
            <input
              value={genericForm.postTitleSelector}
              onChange={(e) => setGenericForm((prev) => ({ ...prev, postTitleSelector: e.target.value }))}
            />
          </label>

          <div className="inline-inputs">
            <label>
              最大列表页
              <input
                type="number"
                min={1}
                value={genericForm.maxListPages}
                onChange={(e) =>
                  setGenericForm((prev) => ({ ...prev, maxListPages: Number(e.target.value) }))
                }
              />
            </label>
            <label>
              最大帖子数
              <input
                type="number"
                min={1}
                value={genericForm.maxPosts}
                onChange={(e) => setGenericForm((prev) => ({ ...prev, maxPosts: Number(e.target.value) }))}
              />
            </label>
            <label>
              请求超时(秒)
              <input
                type="number"
                min={1}
                value={genericForm.requestTimeoutSecs}
                onChange={(e) =>
                  setGenericForm((prev) => ({ ...prev, requestTimeoutSecs: Number(e.target.value) }))
                }
              />
            </label>
            <label>
              最小图片大小(字节)
              <input
                type="number"
                min={0}
                value={genericForm.minImageBytes}
                onChange={(e) =>
                  setGenericForm((prev) => ({ ...prev, minImageBytes: Number(e.target.value) }))
                }
              />
            </label>
          </div>

          <button type="submit" disabled={submitting}>
            {submitting ? '采集中...' : '开始采集'}
          </button>
        </form>
      )}

      {job && (
        <section className="card">
          <h2>任务状态</h2>
          <p>ID: {job.id}</p>
          <p>站点: {job.siteName}</p>
          <p>状态: {job.status}</p>
          <p>
            统计: 帖子 {stats.postCount} / 图片 {stats.photoCount}
          </p>
        </section>
      )}

      {error && <p className="error">{error}</p>}

      {posts.length > 0 && (
        <section className="card">
          <h2>帖子内容</h2>
          <div className="post-list">
            {posts.map((post) => {
              const related = photosByPost.get(post.id) ?? [];
              return (
                <article key={post.id} className="post-item">
                  <h3>
                    <a href={post.url} target="_blank" rel="noreferrer">
                      {post.title}
                    </a>
                  </h3>
                  <p className="content">{post.content || '（无正文）'}</p>
                  {related.length > 0 && (
                    <div className="photo-list">
                      {related.map((photo) => (
                        <div key={photo.id} className="photo-item">
                          <a href={photo.url} target="_blank" rel="noreferrer">
                            {photo.fileName ?? photo.url}
                          </a>
                        </div>
                      ))}
                    </div>
                  )}
                </article>
              );
            })}
          </div>
        </section>
      )}
    </div>
  );
}
