// 创建路由访问权限标记：该文件定义了一个名为 @Public() 的装饰器，用于标记哪些路由端点可以被公开访问，无需身份验证或授权
import { SetMetadata } from "@nestjs/common";

/**
 * 这段代码使用 NestJS 的 SetMetadata 函数创建了一个装饰器，该装饰器会在路由处理程序上设置名为 'isPublic' 的元数据，并赋值为 true。
 * 与守卫配合使用： 通常在项目中，这个装饰器会与一个全局认证守卫（例如 JwtAuthGuard）配合使用。守卫会检查路由处理程序上的元数据，如果发现 isPublic 为 true，则允许请求通过而不进行身份验证检查。
 */
export const IS_PUBLIC_KEY = "isPublic";
export const Public = () => SetMetadata(IS_PUBLIC_KEY, true);

/**
 * 示例
 *
 * import { Controller, Get } from '@nestjs/common';
 * import { Public } from '../core/decorators/public.decorators';
 * @Controller('api')
 * export class SomeController {
 * // 这个路由需要验证身份
 * @Get('protected')
 * getProtectedData() {
 *  return { message: '这是受保护的数据' };
 * }

 * // 这个路由不需要验证身份，任何人都可以访问
 * @Public()
 * @Get('public')
 * getPublicData() {
 * return { message: '这是公开数据' };
 * }
* }
 */
