import { ApiProperty } from "@nestjs/swagger";
import { IsNotEmpty, IsString } from "class-validator";

export class LoginDto {
  @ApiProperty({
    description: "Authorization code for login",
    example: "auth_code_123",
  })
  @IsString()
  @IsNotEmpty()
  code!: string;
}
