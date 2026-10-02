import { Module } from "@nestjs/common";
import { ConfigModule } from "@nestjs/config";
import { AppController } from "./app.controller";

@Module({
  imports: [ConfigModule.forRoot({ ignoreEnvFile: true, isGlobal: true })],
  controllers: [AppController],
})
export class AppModule {}
