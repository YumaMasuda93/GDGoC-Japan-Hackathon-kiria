import { useEffect, useState } from "react";
import "./App.css";

type HealthResponse = {
  status: string;
  timestamp: string;
};

export default function App() {
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [error, setError] = useState<string>("");

  useEffect(() => {
    const load = async () => {
      try {
        const response = await fetch("/api/health");
        if (!response.ok) {
          throw new Error(`request failed: ${response.status}`);
        }

        const body: HealthResponse = await response.json();
        setHealth(body);
      } catch (err) {
        const message = err instanceof Error ? err.message : "unknown error";
        setError(message);
      }
    };

    void load();
  }, []);

  return (
    <main className="app-shell">
      <section className="card">
        <p className="eyebrow">React + TypeScript + Go</p>
        <h1>kiria</h1>
        <p className="lead">
          フロントエンドとバックエンドの最小構成を作成しました。
        </p>

        <div className="status-panel">
          <h2>API Status</h2>
          {health ? (
            <dl>
              <div>
                <dt>Status</dt>
                <dd>{health.status}</dd>
              </div>
              <div>
                <dt>Timestamp</dt>
                <dd>{health.timestamp}</dd>
              </div>
            </dl>
          ) : error ? (
            <p className="error">接続に失敗しました: {error}</p>
          ) : (
            <p>バックエンドに接続しています...</p>
          )}
        </div>
      </section>
    </main>
  );
}
