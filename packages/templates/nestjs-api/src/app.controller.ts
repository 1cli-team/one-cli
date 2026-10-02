import { Controller, Get } from "@nestjs/common";
import { ApiOperation, ApiTags } from "@nestjs/swagger";

@ApiTags("Application")
@Controller()
export class AppController {
  @Get()
  @ApiOperation({ summary: "Application information / 服务信息" })
  info() {
    return { name: "nestjs-api", version: "0.1.0" };
  }

  @Get("health")
  @ApiOperation({ summary: "Liveness check / 存活检查" })
  health() {
    return { status: "ok", uptime: process.uptime() };
  }
}
