import React from "react";
import ReactDOM from "react-dom/client";
import "./styles.css";
import IdentityGate from "./identity/IdentityGate";

class ErrorBoundary extends React.Component<
  { children: React.ReactNode },
  { error: boolean }
> {
  state = { error: false };
  static getDerivedStateFromError() {
    return { error: true };
  }
  render() {
    return this.state.error ? (
      <div className="fatal-screen">
        <img src="/freetower.svg" width="64" />
        <h1>世界连接暂时中断</h1>
        <p>世界进度保存在服务器，请重新进入。</p>
        <button onClick={() => location.reload()}>重新进入世界</button>
      </div>
    ) : (
      this.props.children
    );
  }
}
ReactDOM.createRoot(document.getElementById("root")!).render(
  <ErrorBoundary>
    <React.Suspense fallback={<div>Loading…</div>}>
      <IdentityGate />
    </React.Suspense>
  </ErrorBoundary>,
);
