import { ApiProperty } from "@nestjs/swagger";
import { IsNotEmpty, IsString } from "class-validator";

export class HelloDto {
  @ApiProperty({
    description: "User name for greeting",
    example: "john_doe",
  })
  @IsString()
  @IsNotEmpty()
  user!: string;
}
