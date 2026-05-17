import { ApiProperty } from "@nestjs/swagger";
import { IsInt, IsOptional, Max, Min } from "class-validator";

export class PaginationDto {
  @ApiProperty({ description: "Page number", example: 1, required: false })
  @IsOptional()
  @IsInt()
  @Min(1)
  page: number = 1;

  @ApiProperty({ description: "Items per page", example: 20, required: false })
  @IsOptional()
  @IsInt()
  @Min(1)
  @Max(100)
  pageSize: number = 20;
}

export class PaginatedResult<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;

  constructor(items: T[], total: number, page: number, pageSize: number) {
    this.items = items;
    this.total = total;
    this.page = page;
    this.pageSize = pageSize;
    this.totalPages = Math.ceil(total / pageSize);
  }
}
