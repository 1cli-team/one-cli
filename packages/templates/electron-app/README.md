# electron-template

主进程内置： typeorm+sqlite3、electron-log、electron-store

主进程框架：inversify

主进程打包： esbuild

打包构建： electron-builder

---

渲染进程内置：antd、ahooks、redux、react-router

渲染进程框架： react

渲染进程打包： vite

---

运行项目
```shell
# install
pnpm i

# dev
pnpm run dev

# build
pnpm run release
```

---

主进程目录结构

```
│  app.ts               所有 service 初始化
│  global.d.ts          主进程类型定义
│  index.ts             主进程打包的入口
│  interfaces.ts        service、controller、repository 定义
│  inversify.config.ts  服务绑定
│  main.d.ts            主进程类型定义
│  preload.ts           与渲染进程交互
│  types.ts             所有服务的类型定义	
│  
├─controller
│      HomeController.ts  controller 定义，主要是与渲染进程交互逻辑
│      
├─entity
│      User.ts  实体类
│      
├─helper
│      decorator.ts     ipc 交互的装饰器方法
│      utils.ts         工具文件
│      variables.ts     主进程常量定义
│      
├─repository
│      userRepositoryImpl.ts  实体类资源库
│      
└─services
        DatabaseServiceImpl.ts    数据库服务
        IpcHandlerServiceImpl.ts  ipc 通讯服务
        LoggerServiceImpl.ts      日志服务
        MainWindowServiceImpl.ts  窗口服务
        ProtocolServiceImpl.ts    协议注册
        SessionServiceImpl.ts     electron session
        StoreServiceImpl.ts       软件配置服务
        UpdateServiceImpl.ts      升级服务
```

渲染进程目录结构

```
│  App.scss
│  App.tsx
│  appSlice.ts
│  index.scss
│  main.tsx
│  renderer.d.ts
│  vite-env.d.ts
│  
├─assets
│      .gitkeep
│      
├─components
│      .gitkeep
│      
├─hooks
│      electron.ts
│      
├─nodes
│  └─HomePage
│          index.scss
│          index.tsx
│          
├─store
│      index.ts
│      
└─utils
        http.ts
        index.ts
```


