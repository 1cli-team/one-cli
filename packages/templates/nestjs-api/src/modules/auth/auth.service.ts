import { Injectable, UnauthorizedException } from "@nestjs/common";
import { JwtService } from "@nestjs/jwt";

@Injectable()
export class AuthService {
  constructor(private readonly jwtService: JwtService) {}

  async getAccessToken(code: string) {
    const payload = { userId: code };
    return this.jwtService.sign(payload);
  }

  async refreshAccessToken(token: string) {
    try {
      const payload = this.jwtService.verify(token, { ignoreExpiration: true });
      // 只保留业务字段，去除 JWT 元数据（iat、exp 等），重新签发
      const { iat, exp, nbf, jti, ...cleanPayload } = payload;
      return this.jwtService.sign(cleanPayload);
    } catch {
      throw new UnauthorizedException("Invalid token");
    }
  }

  verifyAccessToken(token: string) {
    return this.jwtService.verify(token);
  }
}
