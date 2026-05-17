import { Controller, Get } from "@nestjs/common";
import { ApiOperation, ApiResponse, ApiTags } from "@nestjs/swagger";
import { Public } from "@/core/decorators/public.decorators";

@ApiTags("app")
@Controller()
export class AppController {
  @ApiOperation({ summary: "Get API information" })
  @ApiResponse({
    status: 200,
    description: "API information retrieved successfully",
    schema: {
      type: "object",
      properties: {
        code: { type: "number", example: 0 },
        message: { type: "string", example: "success" },
        data: {
          type: "object",
          properties: {
            name: { type: "string", example: "NestJS Template API" },
            version: { type: "string", example: "0.1.0" },
            description: {
              type: "string",
              example:
                "A NestJS REST API template with authentication and validation",
            },
            endpoints: {
              type: "object",
              properties: {
                auth: { type: "string", example: "/api/v1/auth" },
                common: { type: "string", example: "/api/v1/common" },
                health: { type: "string", example: "/api/v1" },
              },
            },
          },
        },
      },
    },
  })
  @Public()
  @Get()
  getRoot() {
    return {
      name: "NestJS Template API",
      version: "0.1.0",
      description:
        "A NestJS REST API template with authentication and validation",
      endpoints: {
        auth: "/api/v1/auth",
        common: "/api/v1/common",
        health: "/api/v1",
      },
    };
  }
}
