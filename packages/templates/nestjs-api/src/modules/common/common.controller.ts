import { Body, Controller, HttpCode, Post } from "@nestjs/common";
import {
  ApiBearerAuth,
  ApiBody,
  ApiOperation,
  ApiResponse,
  ApiTags,
} from "@nestjs/swagger";
import { Public } from "@/core/decorators/public.decorators";
import { logger } from "@/shared/utils";
import { CommonService } from "./common.service";
import { HelloDto } from "./dto/hello.dto";

@ApiTags("common")
@Controller("common")
export class CommonController {
  constructor(private readonly commonService: CommonService) {}

  @ApiOperation({ summary: "Get personalized greeting" })
  @ApiBody({ type: HelloDto })
  @ApiResponse({
    status: 200,
    description: "Greeting message retrieved successfully",
    schema: {
      type: "object",
      properties: {
        code: { type: "number", example: 0 },
        message: { type: "string", example: "success" },
        data: { type: "string", example: "Hello john_doe!" },
      },
    },
  })
  @ApiResponse({ status: 401, description: "Unauthorized" })
  @ApiBearerAuth("JWT-auth")
  @Post()
  @HttpCode(200)
  getHello(@Body() body: HelloDto): string {
    const { user } = body;

    logger.info("这是一条 Pino 普通日志");

    return this.commonService.getHello(user);
  }

  @ApiOperation({ summary: "Public test endpoint" })
  @ApiResponse({
    status: 200,
    description: "Public endpoint test",
    schema: {
      type: "object",
      properties: {
        code: { type: "number", example: 0 },
        message: { type: "string", example: "success" },
        data: { type: "string", example: "this endpoint is public" },
      },
    },
  })
  @Post("public")
  @HttpCode(200)
  @Public()
  test() {
    return "this endpoint is public";
  }
}
