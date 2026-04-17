import { FormEvent, useState } from 'react';
import { Link } from 'react-router-dom';
import { Alert, Button, Card, Input, InputNumber, Typography } from 'antd';
import { createCDPJob, getJob } from '../api';
import type { CDPCrawlConfig, CrawlJob } from '../types';

const { Text } = Typography;

const defaultForm = {
  siteName: 'cdp-page',
  startUrl: '',
  remoteDebugUrl: 'http://127.0.0.1:9222',
  pageReadySelector: '',
  contentSelector: 'body',
  imageSelector: 'img',
  titleSelector: 'title',
  waitAfterLoadMs: 1800,
  minImageBytes: 51200,
};

export default function CDPPage() {
  const [form, setForm] = useState(defaultForm);
  const [job, setJob] = useState<CrawlJob | null>(null);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError('');
    setJob(null);
    setSubmitting(true);

    const payload: CDPCrawlConfig = {
      siteName: form.siteName.trim(),
      startUrl: form.startUrl.trim(),
      remoteDebugUrl: form.remoteDebugUrl.trim(),
      pageReadySelector: form.pageReadySelector.trim(),
      contentSelector: form.contentSelector.trim(),
      imageSelector: form.imageSelector.trim(),
      titleSelector: form.titleSelector.trim(),
      waitAfterLoadMs: Number(form.waitAfterLoadMs),
      minImageBytes: Number(form.minImageBytes),
    };

    try {
      const created = await createCDPJob(payload);
      setJob(created);
      await poll(created.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建任务失败');
    } finally {
      setSubmitting(false);
    }
  }

  async function poll(jobId: string): Promise<void> {
    for (let i = 0; i < 600; i += 1) {
      const latest = await getJob(jobId);
      setJob(latest);
      if (latest.status === 'done' || latest.status === 'failed') {
        return;
      }
      await new Promise((resolve) => setTimeout(resolve, 2000));
    }
  }

  return (
    <div className="container">
      <h1>通用网页登录态抓取</h1>
      <p className="subtitle">输入单个网页 URL，后端通过 CDP 在新页面抓取正文和图片。</p>

      <Card className="card">
        <form className="form-grid" onSubmit={onSubmit}>
          <Text type="secondary">
            先启动 Chrome 调试端口并在浏览器里完成登录，再发起任务。默认命令：
            <code> /Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --remote-debugging-port=9222</code>
          </Text>

          <label>
            任务名
            <Input required value={form.siteName} onChange={(e) => setForm((prev) => ({ ...prev, siteName: e.target.value }))} />
          </label>
          <label>
            网页 URL
            <Input required placeholder="https://example.com/page" value={form.startUrl} onChange={(e) => setForm((prev) => ({ ...prev, startUrl: e.target.value }))} />
          </label>
          <label>
            Chrome DevTools 地址
            <Input required value={form.remoteDebugUrl} onChange={(e) => setForm((prev) => ({ ...prev, remoteDebugUrl: e.target.value }))} />
          </label>
          <label>
            页面就绪选择器（可空）
            <Input placeholder="例如：main.article" value={form.pageReadySelector} onChange={(e) => setForm((prev) => ({ ...prev, pageReadySelector: e.target.value }))} />
          </label>
          <label>
            正文根选择器
            <Input value={form.contentSelector} onChange={(e) => setForm((prev) => ({ ...prev, contentSelector: e.target.value }))} />
          </label>
          <label>
            图片选择器
            <Input value={form.imageSelector} onChange={(e) => setForm((prev) => ({ ...prev, imageSelector: e.target.value }))} />
          </label>
          <label>
            标题选择器
            <Input value={form.titleSelector} onChange={(e) => setForm((prev) => ({ ...prev, titleSelector: e.target.value }))} />
          </label>
          <div className="inline-inputs">
            <label>
              额外等待时间(ms)
              <InputNumber min={0} value={form.waitAfterLoadMs} onChange={(v) => setForm((prev) => ({ ...prev, waitAfterLoadMs: Number(v ?? 0) }))} />
            </label>
            <label>
              最小图片大小(字节)
              <InputNumber min={0} value={form.minImageBytes} onChange={(v) => setForm((prev) => ({ ...prev, minImageBytes: Number(v ?? 0) }))} />
            </label>
          </div>

          <Button type="primary" htmlType="submit" loading={submitting}>
            开始抓取
          </Button>
        </form>
      </Card>

      {error && <Alert style={{ marginTop: 16 }} message={error} type="error" showIcon />}

      {job && (
        <Card className="card">
          <div className="form-grid">
            <div>任务 ID: {job.id}</div>
            <div>状态: {job.status}</div>
            {job.error && <Alert message={`错误: ${job.error}`} type="error" showIcon />}
            <Link to={`/viewer/${job.id}`}>去查看抓取结果</Link>
          </div>
        </Card>
      )}
    </div>
  );
}
