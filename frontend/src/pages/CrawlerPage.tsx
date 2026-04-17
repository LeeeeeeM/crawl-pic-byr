import { FormEvent, useMemo, useState } from 'react';
import { Alert, Button, Card, Input, InputNumber, Tabs, Typography } from 'antd';
import { createBYRJob, createJob, getJob, getPhotos, getPosts } from '../api';
import type { BYRCrawlConfig, CrawlConfig, CrawlJob, Photo, Post } from '../types';

const { TextArea } = Input;
const { Text, Link } = Typography;

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

export default function CrawlerPage() {
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

      <Tabs
        activeKey={tab}
        onChange={(key) => setTab(key as 'byr' | 'generic')}
        items={[
          {
            key: 'byr',
            label: 'BYR 登录态爬取',
            children: (
              <Card className="card">
                <form className="form-grid" onSubmit={onBYRSubmit}>
                  <Text type="secondary">
                    先启动 Chrome 调试端口并手动登录 BYR，再提交任务。默认命令：
                    <code> /Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --remote-debugging-port=9222</code>
                  </Text>
                  <Text type="secondary">图片过滤默认阈值为 50KB（51200 字节）。</Text>

                  <label>
                    任务名
                    <Input required value={byrForm.siteName} onChange={(e) => setByrForm((prev) => ({ ...prev, siteName: e.target.value }))} />
                  </label>

                  <label>
                    板块名
                    <Input required value={byrForm.boardName} onChange={(e) => setByrForm((prev) => ({ ...prev, boardName: e.target.value }))} />
                  </label>

                  <div className="inline-inputs">
                    <label>
                      起始页
                      <InputNumber min={1} value={byrForm.startPage} onChange={(v) => setByrForm((prev) => ({ ...prev, startPage: Number(v ?? 1) }))} />
                    </label>
                    <label>
                      最大页数
                      <InputNumber min={1} value={byrForm.maxPages} onChange={(v) => setByrForm((prev) => ({ ...prev, maxPages: Number(v ?? 1) }))} />
                    </label>
                    <label>
                      最小图片大小(字节)
                      <InputNumber min={0} value={byrForm.minImageBytes} onChange={(v) => setByrForm((prev) => ({ ...prev, minImageBytes: Number(v ?? 0) }))} />
                    </label>
                  </div>

                  <label>
                    Chrome DevTools 地址
                    <Input required value={byrForm.remoteDebugUrl} onChange={(e) => setByrForm((prev) => ({ ...prev, remoteDebugUrl: e.target.value }))} />
                  </label>

                  <Button type="primary" htmlType="submit" loading={submitting}>
                    开始 BYR 爬取
                  </Button>
                </form>
              </Card>
            ),
          },
          {
            key: 'generic',
            label: '通用选择器爬取',
            children: (
              <Card className="card">
                <form className="form-grid" onSubmit={onGenericSubmit}>
                  <Text type="secondary">图片过滤默认阈值为 50KB（51200 字节）。</Text>

                  <label>
                    站点名称
                    <Input required value={genericForm.siteName} onChange={(e) => setGenericForm((prev) => ({ ...prev, siteName: e.target.value }))} />
                  </label>

                  <label>
                    起始列表页（每行一个 URL）
                    <TextArea required rows={3} value={genericForm.startUrls} onChange={(e) => setGenericForm((prev) => ({ ...prev, startUrls: e.target.value }))} />
                  </label>

                  <label>
                    允许域名（每行一个，如 example.com）
                    <TextArea rows={2} value={genericForm.allowedDomains} onChange={(e) => setGenericForm((prev) => ({ ...prev, allowedDomains: e.target.value }))} />
                  </label>

                  <label>
                    帖子链接选择器
                    <Input required value={genericForm.postLinkSelector} onChange={(e) => setGenericForm((prev) => ({ ...prev, postLinkSelector: e.target.value }))} />
                  </label>

                  <label>
                    下一页选择器（可空）
                    <Input value={genericForm.nextPageSelector} onChange={(e) => setGenericForm((prev) => ({ ...prev, nextPageSelector: e.target.value }))} />
                  </label>

                  <label>
                    图片选择器
                    <Input value={genericForm.imageSelector} onChange={(e) => setGenericForm((prev) => ({ ...prev, imageSelector: e.target.value }))} />
                  </label>

                  <label>
                    标题选择器
                    <Input value={genericForm.postTitleSelector} onChange={(e) => setGenericForm((prev) => ({ ...prev, postTitleSelector: e.target.value }))} />
                  </label>

                  <div className="inline-inputs">
                    <label>
                      最大列表页
                      <InputNumber min={1} value={genericForm.maxListPages} onChange={(v) => setGenericForm((prev) => ({ ...prev, maxListPages: Number(v ?? 1) }))} />
                    </label>
                    <label>
                      最大帖子数
                      <InputNumber min={1} value={genericForm.maxPosts} onChange={(v) => setGenericForm((prev) => ({ ...prev, maxPosts: Number(v ?? 1) }))} />
                    </label>
                    <label>
                      请求超时(秒)
                      <InputNumber min={1} value={genericForm.requestTimeoutSecs} onChange={(v) => setGenericForm((prev) => ({ ...prev, requestTimeoutSecs: Number(v ?? 1) }))} />
                    </label>
                    <label>
                      最小图片大小(字节)
                      <InputNumber min={0} value={genericForm.minImageBytes} onChange={(v) => setGenericForm((prev) => ({ ...prev, minImageBytes: Number(v ?? 0) }))} />
                    </label>
                  </div>

                  <Button type="primary" htmlType="submit" loading={submitting}>
                    开始采集
                  </Button>
                </form>
              </Card>
            ),
          },
        ]}
      />

      {job && (
        <Card className="card" title="任务状态">
          <p>ID: {job.id}</p>
          <p>站点: {job.siteName}</p>
          <p>状态: {job.status}</p>
          <p>统计: 帖子 {stats.postCount} / 图片 {stats.photoCount}</p>
        </Card>
      )}

      {error && <Alert style={{ marginTop: 16 }} message={error} type="error" showIcon />}

      {posts.length > 0 && (
        <Card className="card" title="帖子内容">
          <div className="post-list">
            {posts.map((post) => {
              const related = photosByPost.get(post.id) ?? [];
              return (
                <article key={post.id} className="post-item">
                  <h3>
                    <Link href={post.url} target="_blank" rel="noreferrer">
                      {post.title}
                    </Link>
                  </h3>
                  <p className="content">{post.content || '（无正文）'}</p>
                  {related.length > 0 && (
                    <div className="photo-list">
                      {related.map((photo) => (
                        <div key={photo.id} className="photo-item">
                          <Link href={photo.url} target="_blank" rel="noreferrer">
                            {photo.fileName ?? photo.url}
                          </Link>
                        </div>
                      ))}
                    </div>
                  )}
                </article>
              );
            })}
          </div>
        </Card>
      )}
    </div>
  );
}
