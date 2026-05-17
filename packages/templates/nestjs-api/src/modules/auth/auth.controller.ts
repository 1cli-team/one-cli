import { Body, Controller, Post } from "@nestjs/common";
import {
  ApiBearerAuth,
  ApiBody,
  ApiOperation,
  ApiResponse,
  ApiTags,
} from "@nestjs/swagger";
import { Public } from "@/core/decorators/public.decorators";
import { AuthService } from "./auth.service";
import { LoginDto } from "./dto/login.dto";
import { RefreshDto } from "./dto/refresh.dto";

@ApiTags("auth")
@Controller("auth")
export class AuthController {
  constructor(private readonly authService: AuthService) {}

  @ApiOperation({ summary: "User login" })
  @ApiBody({ type: LoginDto })
  @ApiResponse({
    status: 200,
    description: "Login successful",
    schema: {
      type: "object",
      properties: {
        code: { type: "number", example: 0 },
        message: { type: "string", example: "success" },
        data: {
          type: "string",
          example: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
        },
      },
    },
  })
  @ApiResponse({ status: 400, description: "Bad request" })
  @Public()
  @Post("login")
  async login(@Body() body: LoginDto) {
    const { code } = body;

    return this.authService.getAccessToken(code);
  }

  @ApiOperation({ summary: "Refresh access token" })
  @ApiBody({ type: RefreshDto })
  @ApiResponse({
    status: 200,
    description: "Token refreshed successfully",
    schema: {
      type: "object",
      properties: {
        code: { type: "number", example: 0 },
        message: { type: "string", example: "success" },
        data: {
          type: "string",
          example: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
        },
      },
    },
  })
  @ApiResponse({ status: 401, description: "Unauthorized" })
  @ApiBearerAuth("JWT-auth")
  @Post("refresh")
  async refresh(@Body() body: RefreshDto) {
    const { token } = body;

    return this.authService.refreshAccessToken(token);
  }
}
