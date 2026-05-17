import { Injectable } from "@nestjs/common";

@Injectable()
export class CommonService {
  getHello(user: string): string {
    return `Hello ${user}!`;
  }
}
