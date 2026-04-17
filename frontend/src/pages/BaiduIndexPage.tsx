import { FormEvent, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { Alert, Button, Card, Input, InputNumber, Select, Typography } from 'antd';
import { API_BASE_URL, createBaiduIndexJob, getJob, getPhotos, getPosts } from '../api';
import type { BaiduIndexCrawlConfig, CrawlJob, Photo, Post } from '../types';

const { Text } = Typography;

const periodOptions = [
  { value: '7d', label: '近7天' },
  { value: '30d', label: '近30天' },
  { value: '90d', label: '近90天' },
  { value: '180d', label: '近半年' },
  { value: 'all', label: '全部' },
] as const;

const defaultForm: {
  siteName: string;
  keyword: string;
  startUrl: string;
  period: BaiduIndexCrawlConfig['period'];
  remoteDebugUrl: string;
  waitAfterLoadMs: number;
  minImageBytes: number;
} = {
  siteName: '',
  keyword: '',
  startUrl: '',
  period: '30d',
  remoteDebugUrl: 'http://127.0.0.1:9222',
  waitAfterLoadMs: 1800,
  minImageBytes: 10240,
};

export default function BaiduIndexPage() {
  const [form, setForm] = useState(defaultForm);
  const [job, setJob] = useState<CrawlJob | null>(null);
  const [posts, setPosts] = useState<Post[]>([]);
  const [photos, setPhotos] = useState<Photo[]>([]);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [viewerImages, setViewerImages] = useState<Array<{ resolvedUrl: string; name: string }>>([]);
  const [viewerIndex, setViewerIndex] = useState(0);
  const [viewerImageLoading, setViewerImageLoading] = useState(false);

  const assetPhotos = useMemo(() => {
    return photos.map((p) => ({
      ...p,
      resolvedUrl: p.url.startsWith('http://') || p.url.startsWith('https://') ? p.url : `${API_BASE_URL}${p.url}`,
    }));
  }, [photos]);

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (viewerImages.length === 0) {
        return;
      }
      if (e.key === 'Escape') {
        closeViewer();
      } else if (e.key === 'ArrowLeft') {
        prevImage();
      } else if (e.key === 'ArrowRight') {
        nextImage();
      }
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [viewerImages, viewerIndex]);

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError('');
    setSubmitting(true);
    setPosts([]);
    setPhotos([]);

    const payload: BaiduIndexCrawlConfig = {
      siteName: form.siteName.trim() || `baidu-index-${form.keyword.trim()}`,
      keyword: form.keyword.trim(),
      startUrl: form.startUrl.trim(),
      period: form.period,
      remoteDebugUrl: form.remoteDebugUrl.trim(),
      waitAfterLoadMs: Number(form.waitAfterLoadMs),
      minImageBytes: Number(form.minImageBytes),
    };

    try {
      const created = await createBaiduIndexJob(payload);
      setJob(created);
      await pollJob(created.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建任务失败');
    } finally {
      setSubmitting(false);
    }
  }

  async function pollJob(jobId: string): Promise<void> {
    for (let i = 0; i < 120; i += 1) {
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

    setError('轮询超时，请到结果页查看');
  }

  function openViewer(images: Array<{ resolvedUrl: string; name: string }>, index: number) {
    setViewerImages(images);
    setViewerIndex(index);
    setViewerImageLoading(true);
  }

  function closeViewer() {
    setViewerImages([]);
    setViewerIndex(0);
    setViewerImageLoading(false);
  }

  function prevImage() {
    if (viewerImages.length === 0) {
      return;
    }
    setViewerIndex((prev) => (prev === 0 ? viewerImages.length - 1 : prev - 1));
    setViewerImageLoading(true);
  }

  function nextImage() {
    if (viewerImages.length === 0) {
      return;
    }
    setViewerIndex((prev) => (prev === viewerImages.length - 1 ? 0 : prev + 1));
    setViewerImageLoading(true);
  }

  return (
    <div className="container">
      <h1>Baidu 指数趋势抓取</h1>
      <p className="subtitle">支持近7天/近30天/近90天/近半年/全部。若需要登录，将等待 1 分钟，超时自动结束。</p>

      <Card className="card">
        <form className="form-grid" onSubmit={onSubmit}>
          <Text type="secondary">
            先启动 Chrome 调试端口并在浏览器里完成登录。默认命令：
            <code> /Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --remote-debugging-port=9222</code>
          </Text>
          <label>
            搜索词
            <Input required value={form.keyword} onChange={(e) => setForm((prev) => ({ ...prev, keyword: e.target.value }))} />
          </label>
          <label>
            自定义趋势页 URL
            <Input
              placeholder="https://index.baidu.com/v2/main/index.html#/trend/openclaw?words=openclaw"
              value={form.startUrl}
              onChange={(e) => setForm((prev) => ({ ...prev, startUrl: e.target.value }))}
              disabled
            />
          </label>
          <label>
            任务名（可空）
            <Input value={form.siteName} onChange={(e) => setForm((prev) => ({ ...prev, siteName: e.target.value }))} />
          </label>
          <label>
            Chrome DevTools 地址
            <Input required value={form.remoteDebugUrl} onChange={(e) => setForm((prev) => ({ ...prev, remoteDebugUrl: e.target.value }))} />
          </label>
          <div className="inline-inputs">
            <label>
              时间段
              <Select
                value={form.period}
                options={periodOptions as unknown as { value: string; label: string }[]}
                onChange={(v) => setForm((prev) => ({ ...prev, period: v as BaiduIndexCrawlConfig['period'] }))}
              />
            </label>
            <label>
              渲染后额外等待(ms)
              <InputNumber min={0} value={form.waitAfterLoadMs} onChange={(v) => setForm((prev) => ({ ...prev, waitAfterLoadMs: Number(v ?? 0) }))} />
            </label>
            <label>
              最小图片大小(字节)
              <InputNumber min={0} value={form.minImageBytes} onChange={(v) => setForm((prev) => ({ ...prev, minImageBytes: Number(v ?? 0) }))} />
            </label>
          </div>
          <Button type="primary" htmlType="submit" loading={submitting}>开始抓取趋势图</Button>
        </form>
      </Card>

      {error && <Alert style={{ marginTop: 16 }} message={error} type="error" showIcon />}

      {job && (
        <Card className="card">
          <div className="form-grid">
            <div>任务 ID: {job.id}</div>
            <div>状态: {job.status}</div>
            {job.error && <Alert message={`错误: ${job.error}`} type="error" showIcon />}
            <Link to={`/viewer/${job.id}`}>在主贴展示页查看记录</Link>
          </div>
        </Card>
      )}

      {posts.length > 0 && (
        <Card className="card" title="抓取结果">
          <div className="post-list">
            {posts.map((post) => (
              <article key={post.id} className="post-item">
                <h3>{post.title}</h3>
                <p className="content">{post.content}</p>
                <a href={post.url} target="_blank" rel="noreferrer">
                  打开来源页
                </a>
                <div className="image-grid">
                  {(() => {
                    const related = assetPhotos.filter((p) => p.postId === post.id);
                    const viewerList = related.map((p) => ({
                      resolvedUrl: p.resolvedUrl,
                      name: p.fileName ?? p.url,
                    }));
                    return related.map((photo, index) => (
                      <button key={photo.id} type="button" className="image-card" onClick={() => openViewer(viewerList, index)}>
                        <img src={photo.resolvedUrl} alt={photo.altText ?? photo.url} loading="lazy" />
                        <span>{photo.fileName ?? photo.url}</span>
                      </button>
                    ));
                  })()}
                </div>
              </article>
            ))}
          </div>
        </Card>
      )}

      {viewerImages.length > 0 && (
        <div className="viewer-mask" onClick={closeViewer}>
          <div className="viewer-panel" onClick={(e) => e.stopPropagation()}>
            <button type="button" className="viewer-close" onClick={closeViewer}>
              关闭
            </button>
            <button type="button" className="viewer-arrow left" onClick={prevImage}>
              ‹
            </button>
            <div className="viewer-stage">
              {viewerImageLoading && <div className="viewer-loading">图片加载中...</div>}
              <img
                className={`viewer-image ${viewerImageLoading ? 'hidden' : ''}`}
                src={viewerImages[viewerIndex].resolvedUrl}
                alt={viewerImages[viewerIndex].name}
                onLoad={() => setViewerImageLoading(false)}
                onError={() => setViewerImageLoading(false)}
              />
            </div>
            <button type="button" className="viewer-arrow right" onClick={nextImage}>
              ›
            </button>
            <div className="viewer-caption">
              <span>
                {viewerIndex + 1} / {viewerImages.length}
              </span>
              <span className="viewer-post-title">{viewerImages[viewerIndex].name}</span>
              <div className="viewer-links">
                <a href={viewerImages[viewerIndex].resolvedUrl} target="_blank" rel="noreferrer">
                  打开原图
                </a>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
