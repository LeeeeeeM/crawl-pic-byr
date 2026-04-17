import { Navigate, NavLink, Outlet, Route, Routes } from 'react-router-dom';
import HomePage from './pages/HomePage';
import CrawlerPage from './pages/CrawlerPage';
import ViewerPage from './pages/ViewerPage';
import CDPPage from './pages/CDPPage';
import BaiduIndexPage from './pages/BaiduIndexPage';

function Layout() {
  return (
    <div className="app-shell">
      <aside className="side-nav">
        <h2 className="side-title">Crawl Pic</h2>
        <nav className="side-links">
          <NavLink to="/" end className={({ isActive }) => (isActive ? 'active' : '')}>
            首页
          </NavLink>
          <NavLink to="/crawler" className={({ isActive }) => (isActive ? 'active' : '')}>
            任务创建
          </NavLink>
          <NavLink to="/cdp" className={({ isActive }) => (isActive ? 'active' : '')}>
            通用CDP
          </NavLink>
          <NavLink to="/baidu-index" className={({ isActive }) => (isActive ? 'active' : '')}>
            Baidu指数
          </NavLink>
          <NavLink to="/viewer" className={({ isActive }) => (isActive ? 'active' : '')}>
            主贴展示
          </NavLink>
        </nav>
      </aside>
      <main className="main-content">
        <Outlet />
      </main>
    </div>
  );
}

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<HomePage />} />
        <Route path="crawler" element={<CrawlerPage />} />
        <Route path="cdp" element={<CDPPage />} />
        <Route path="baidu-index" element={<BaiduIndexPage />} />
        <Route path="viewer" element={<ViewerPage />} />
        <Route path="viewer/:jobId" element={<ViewerPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
