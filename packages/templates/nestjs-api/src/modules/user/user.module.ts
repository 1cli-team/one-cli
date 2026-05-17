// 为了让 NestJS 能够识别并加载以上组件，需要将各层模块组合到一个模块中，并在根模块中引入。
// 在根模块中引入 UserModule，以便 NestJS 能够加载 UserModule 中的组件。
/**
    import { Module } from '@nestjs/common';
    import { UserModule } from './modules/user/user.module';
    // 其它模块，如 AuthModule、CoreModule 等

    @Module({
      imports: [UserModule],
    })
    export class AppModule {}
 */
import { Module } from "@nestjs/common";
import { UserController } from "./user.controller";
import { UserService } from "./user.service";
import { UserRepository } from "../../dao/repositories/user.repository";

@Module({
  controllers: [UserController],
  providers: [UserService, UserRepository],
})
export class UserModule {}
