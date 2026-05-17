import { AxiosResponse } from "axios";

export class AxiosRespError extends Error {
  response?: AxiosResponse<any, any>;

  constructor(message: string, response?: AxiosResponse<any, any>) {
    super(message);
    this.response = response;
    // 维护原型链
    Object.setPrototypeOf(this, AxiosRespError.prototype);
  }
}

export class TokenExpiredError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "TokenExpiredError";
  }
}
