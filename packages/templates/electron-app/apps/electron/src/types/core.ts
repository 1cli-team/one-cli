// 框架契约：所有控制器实现 Controller（空接口，只作类型标记）
export interface Controller {}

// Router 只暴露 init()，编排器启动时调用一次完成 IPC 绑定
export interface AppRouter {
  init(): void;
}
