// DI 符号表：用 Symbol.for 保证跨模块唯一
// 所有"多实例同类型"的注入点要在这里登记（比如 Controller 通过 multiInject 聚合）
export const TYPES = {
  Controller: Symbol.for("Controller"),
};
