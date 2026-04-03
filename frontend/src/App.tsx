import { Navigate, Route, Routes, Link } from 'react-router-dom';
import CrawlerPage from './pages/CrawlerPage';
import ViewerPage from './pages/ViewerPage';

export default function App() {
  return (
    <Routes>
      <Route
        path="/"
        element={
          <>
            <header className="top-nav">
              <Link to="/">任务创建</Link>
              <Link to="/viewer">主贴展示</Link>
            </header>
            <CrawlerPage />
          </>
        }
      />
      <Route
        path="/viewer"
        element={
          <>
            <header className="top-nav">
              <Link to="/">任务创建</Link>
              <Link to="/viewer">主贴展示</Link>
            </header>
            <ViewerPage />
          </>
        }
      />
      <Route
        path="/viewer/:jobId"
        element={
          <>
            <header className="top-nav">
              <Link to="/">任务创建</Link>
              <Link to="/viewer">主贴展示</Link>
            </header>
            <ViewerPage />
          </>
        }
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
