// 定义核心领域模型，这里我们用一个简单的 User 实体来表示业务对象。
export class User {
  constructor(
    public readonly id: string,
    public name: string,
  ) {}
}
