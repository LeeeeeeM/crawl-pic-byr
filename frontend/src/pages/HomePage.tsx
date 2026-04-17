import { Link } from 'react-router-dom';

export default function HomePage() {
  return (
    <div className="container">
      <h1>Crawl Pic</h1>
      <p className="subtitle">一个支持通用网页、BYR、百度指数的图片与内容抓取工具。</p>

      <section className="card">
        <h2>主要功能</h2>
        <ul>
          <li>异步创建任务并跟踪状态（pending/running/done/failed）</li>
          <li>通用选择器抓取帖子和图片</li>
          <li>CDP 登录态抓取（适合动态页面）</li>
          <li>百度指数趋势图抓取（搜索/咨询双图）</li>
          <li>任务结果查看与图片预览 Viewer</li>
        </ul>
      </section>

      <section className="card">
        <h2>快速入口</h2>
        <div className="inline-inputs">
          <Link to="/crawler">任务创建</Link>
          <Link to="/cdp">通用CDP</Link>
          <Link to="/baidu-index">Baidu指数</Link>
          <Link to="/viewer">主贴展示</Link>
        </div>
      </section>
    </div>
  );
}

