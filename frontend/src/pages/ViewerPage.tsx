import { CSSProperties, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { Alert, Button, Card, Input, Modal } from 'antd';
import { cancelJob, deleteJob, getJob, getPhotos, getPosts, listJobs } from '../api';
import type { CrawlJob, Photo, Post } from '../types';

export default function ViewerPage() {
  const navigate = useNavigate();
  const { jobId: routeJobId } = useParams();

  const [jobIdInput, setJobIdInput] = useState(routeJobId ?? '');
  const [jobs, setJobs] = useState<CrawlJob[]>([]);
  const [job, setJob] = useState<CrawlJob | null>(null);
  const [posts, setPosts] = useState<Post[]>([]);
  const [photos, setPhotos] = useState<Photo[]>([]);
  const [viewerPostId, setViewerPostId] = useState<number | null>(null);
  const [viewerIndex, setViewerIndex] = useState(0);
  const [viewOriginalScale, setViewOriginalScale] = useState(true);
  const [viewerImageLoading, setViewerImageLoading] = useState(false);
  const [stageSize, setStageSize] = useState({ width: 0, height: 0 });
  const [naturalSize, setNaturalSize] = useState({ width: 0, height: 0 });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const stageRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    void loadRecentJobs();
    if (routeJobId) {
      setJobIdInput(routeJobId);
      void load(routeJobId);
    }
  }, [routeJobId]);

  useEffect(() => {
    setNaturalSize({ width: 0, height: 0 });
    if (viewerPostId !== null && viewerImages.length > 0) {
      setViewerImageLoading(true);
    }
  }, [viewerIndex, viewerPostId]);

  useEffect(() => {
    if (!stageRef.current) {
      return;
    }
    const el = stageRef.current;
    const update = () => {
      const rect = el.getBoundingClientRect();
      setStageSize({ width: rect.width, height: rect.height });
    };
    update();

    const observer = new ResizeObserver(() => update());
    observer.observe(el);
    return () => observer.disconnect();
  }, [viewerPostId]);

  async function loadRecentJobs() {
    try {
      const loaded = await listJobs(100);
      setJobs(loaded);
    } catch {
      setJobs([]);
    }
  }

  async function onCancelJob(id: string) {
    try {
      await cancelJob(id);
      await loadRecentJobs();
      if (job?.id === id) {
        await load(id);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '停止任务失败');
    }
  }

  async function onDeleteJob(id: string) {
    const ok = await new Promise<boolean>((resolve) => {
      Modal.confirm({
        title: '确认删除任务',
        content: '删除任务后会同时删除该任务的帖子和图片记录，确认删除吗？',
        okText: '确认删除',
        cancelText: '取消',
        onOk: () => resolve(true),
        onCancel: () => resolve(false),
      });
    });
    if (!ok) return;
    try {
      await deleteJob(id);
      await loadRecentJobs();
      if (job?.id === id) {
        setJob(null);
        setPosts([]);
        setPhotos([]);
        setViewerPostId(null);
        setViewerIndex(0);
        setJobIdInput('');
        navigate('/viewer');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除任务失败');
    }
  }

  const photosByPost = useMemo(() => {
    const map = new Map<number, Photo[]>();
    for (const photo of photos) {
      const list = map.get(photo.postId) ?? [];
      list.push(photo);
      map.set(photo.postId, list);
    }
    return map;
  }, [photos]);

  const viewerPostIds = useMemo(
    () => posts.filter((post) => (photosByPost.get(post.id)?.length ?? 0) > 0).map((post) => post.id),
    [posts, photosByPost],
  );

  const viewerPostCursor = useMemo(
    () => (viewerPostId === null ? -1 : viewerPostIds.indexOf(viewerPostId)),
    [viewerPostId, viewerPostIds],
  );

  const viewerPost = useMemo(
    () => (viewerPostId === null ? null : posts.find((post) => post.id === viewerPostId) ?? null),
    [viewerPostId, posts],
  );

  const viewerImages = useMemo(
    () => (viewerPostId === null ? [] : photosByPost.get(viewerPostId) ?? []),
    [photosByPost, viewerPostId],
  );

  useEffect(() => {
    if (viewerPostId === null) {
      return;
    }
    if (!viewerPostIds.includes(viewerPostId)) {
      setViewerPostId(null);
      setViewerIndex(0);
    }
  }, [viewerPostId, viewerPostIds]);

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (viewerPostId === null || viewerImages.length === 0) {
        return;
      }
      if (e.key === 'Escape') {
        closeViewer();
      } else if (e.key === 'ArrowLeft') {
        prevImage();
      } else if (e.key === 'ArrowRight') {
        nextImage();
      } else if (e.key === 'ArrowUp') {
        prevPost();
      } else if (e.key === 'ArrowDown') {
        nextPost();
      }
    }

    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [viewerImages, viewerIndex, viewerPostId, viewerPostCursor]);

  async function load(jobId: string) {
    const trimmed = jobId.trim();
    if (!trimmed) {
      setError('请输入任务 ID');
      return;
    }

    setLoading(true);
    setError('');
    setJob(null);
    setPosts([]);
    setPhotos([]);
    setViewerPostId(null);
    setViewerIndex(0);

    try {
      const [loadedJob, loadedPosts, loadedPhotos] = await Promise.all([
        getJob(trimmed),
        getPosts(trimmed),
        getPhotos(trimmed),
      ]);

      setJob(loadedJob);
      setPosts(loadedPosts);
      setPhotos(loadedPhotos);
      await loadRecentJobs();
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }

  function onOpen() {
    const trimmed = jobIdInput.trim();
    if (!trimmed) {
      setError('请输入任务 ID');
      return;
    }
    navigate(`/viewer/${trimmed}`);
  }

  function openViewer(postId: number, index: number) {
    setViewerPostId(postId);
    setViewerIndex(index);
    setViewerImageLoading(true);
  }

  function closeViewer() {
    setViewerPostId(null);
    setViewerIndex(0);
    setViewerImageLoading(false);
  }

  function prevImage() {
    if (viewerImages.length === 0) {
      return;
    }
    setViewerIndex((prev) => (prev === 0 ? viewerImages.length - 1 : prev - 1));
  }

  function nextImage() {
    if (viewerImages.length === 0) {
      return;
    }
    setViewerIndex((prev) => (prev === viewerImages.length - 1 ? 0 : prev + 1));
  }

  function prevPost() {
    if (viewerPostCursor < 0 || viewerPostIds.length === 0) {
      return;
    }
    const nextCursor = viewerPostCursor === 0 ? viewerPostIds.length - 1 : viewerPostCursor - 1;
    setViewerPostId(viewerPostIds[nextCursor]);
    setViewerIndex(0);
  }

  function nextPost() {
    if (viewerPostCursor < 0 || viewerPostIds.length === 0) {
      return;
    }
    const nextCursor = viewerPostCursor === viewerPostIds.length - 1 ? 0 : viewerPostCursor + 1;
    setViewerPostId(viewerPostIds[nextCursor]);
    setViewerIndex(0);
  }

  function rawImageStyle(): CSSProperties {
    if (!viewOriginalScale) {
      return {};
    }
    if (naturalSize.width <= 0 || naturalSize.height <= 0 || stageSize.width <= 0 || stageSize.height <= 0) {
      return {};
    }
    const scale = Math.min(stageSize.width / naturalSize.width, stageSize.height / naturalSize.height, 1);
    return {
      width: `${Math.floor(naturalSize.width * scale)}px`,
      height: `${Math.floor(naturalSize.height * scale)}px`,
    };
  }

  return (
    <div className="container">
      <h1>主贴与图片展示</h1>
      <p className="subtitle">输入任务 ID，查看每个主贴的正文和图片</p>

      <section className="card viewer-filter">
        <Input
          placeholder="任务 ID"
          value={jobIdInput}
          onChange={(e) => setJobIdInput(e.target.value)}
        />
        <Button type="primary" onClick={onOpen} loading={loading}>
          {loading ? '加载中...' : '打开'}
        </Button>
      </section>

      {error && <Alert message={error} type="error" showIcon style={{ marginTop: 16 }} />}

      {jobs.length > 0 && (
        <Card className="card" title="最近任务">
          <div className="job-list">
            {jobs.map((item) => (
              <div key={item.id} className="job-row">
                <Button type="text" className="job-open" onClick={() => navigate(`/viewer/${item.id}`)}>
                  <span className="job-title">{item.siteName}</span>
                  <span className={`job-status status-${item.status}`}>{item.status}</span>
                  <span className="job-id">{item.id}</span>
                </Button>
                <div className="job-actions">
                  {item.status === 'running' && (
                    <Button type="default" className="job-cancel" onClick={() => void onCancelJob(item.id)}>
                      停止任务
                    </Button>
                  )}
                  <Button type="default" className="job-delete" onClick={() => void onDeleteJob(item.id)}>
                    删除任务
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </Card>
      )}

      {job && (
        <section className="card">
          <h2>任务信息</h2>
          <p>ID: {job.id}</p>
          <p>站点: {job.siteName}</p>
          <p>状态: {job.status}</p>
          <p>
            统计: 主贴 {posts.length} / 图片 {photos.length}
          </p>
        </section>
      )}

      {posts.length > 0 && (
        <section className="card">
          <h2>主贴列表</h2>
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
                    <div className="image-grid">
                      {related.map((photo, index) => (
                        <button
                          key={photo.id}
                          type="button"
                          className="image-card"
                          onClick={() => openViewer(post.id, index)}
                        >
                          <img src={photo.url} alt={photo.altText ?? photo.fileName ?? 'post image'} loading="lazy" />
                          <span>{photo.fileName ?? '查看原图'}</span>
                        </button>
                      ))}
                    </div>
                  )}
                </article>
              );
            })}
          </div>
        </section>
      )}

      {viewerPostId !== null && viewerImages.length > 0 && (
        <div className="viewer-mask" onClick={closeViewer}>
          <div className="viewer-panel" onClick={(e) => e.stopPropagation()}>
            <button type="button" className="viewer-close" onClick={closeViewer}>
              关闭
            </button>
            <button type="button" className="viewer-arrow left" onClick={prevImage}>
              ‹
            </button>
            <div ref={stageRef} className={`viewer-stage ${viewOriginalScale ? 'raw' : ''}`}>
              {viewerImageLoading && <div className="viewer-loading">图片加载中...</div>}
              <img
                className={`viewer-image ${viewOriginalScale ? 'raw' : ''} ${viewerImageLoading ? 'hidden' : ''}`}
                src={viewerImages[viewerIndex].url}
                alt={viewerImages[viewerIndex].altText ?? viewerImages[viewerIndex].fileName ?? 'preview'}
                style={rawImageStyle()}
                onLoad={(e) => {
                  setNaturalSize({
                    width: e.currentTarget.naturalWidth,
                    height: e.currentTarget.naturalHeight,
                  });
                  setViewerImageLoading(false);
                }}
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
              <button type="button" className="viewer-toggle" onClick={prevPost}>
                上一帖
              </button>
              <button type="button" className="viewer-toggle" onClick={nextPost}>
                下一帖
              </button>
              <button
                type="button"
                className="viewer-toggle"
                onClick={() => setViewOriginalScale((prev) => !prev)}
              >
                {viewOriginalScale ? '适配显示' : '原始比例'}
              </button>
              <div className="viewer-links">
                {viewerPost?.url && (
                  <a href={viewerPost.url} target="_blank" rel="noreferrer">
                    打开原帖
                  </a>
                )}
                <a href={viewerImages[viewerIndex].url} target="_blank" rel="noreferrer">
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
