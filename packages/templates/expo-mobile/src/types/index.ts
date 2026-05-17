export interface AppResponse<T> {
  code: number;
  message: string;
  data: T;
}
