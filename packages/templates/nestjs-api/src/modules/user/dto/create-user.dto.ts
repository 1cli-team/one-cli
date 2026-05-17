// DTO 的作用：用于数据校验、转换和格式化，保障接口数据的一致性和安全性。
// 目录结构调整：建议在每个业务模块内部增加 dto 文件夹，和其他层（controllers、services、domain、repositories）并列，既保持了分层架构的清晰，又便于维护和扩展。

import { ApiProperty } from "@nestjs/swagger";
import { IsString, Length } from "class-validator";

export class CreateUserDto {
  @ApiProperty({
    description: "User name",
    example: "john_doe",
    minLength: 2,
    maxLength: 50,
  })
  @IsString()
  @Length(2, 50)
  name!: string;
}
