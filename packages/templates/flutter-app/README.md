# one-template-flutter-app

Flutter app in a native pub workspace. The Flutter SDK is pinned in the root `mise.toml`; Dart comes with Flutter. One prepares the shared dependencies before running tasks. Commit the root `pubspec.lock` after dependency changes.

Flutter 应用，使用原生 pub workspace。Flutter SDK 版本固定在根目录 `mise.toml`，Dart 随 Flutter 提供。One 在执行任务前准备共享依赖，依赖变化后请提交根目录 `pubspec.lock`。

```sh
one dev -p one-template-flutter-app -- -d chrome
one test -p one-template-flutter-app
one build -p one-template-flutter-app
```

The Web entry point is ready to use. To generate native runners, run the explicit `platforms` task; it uses `flutter create` without `--overwrite`. Review and commit the generated platform files. Android requires its SDK; iOS/macOS require Xcode. Signing and deployment remain explicit platform operations.

Web 入口开箱可用。通过 `platforms` 任务生成原生平台工程；它调用不带 `--overwrite` 的 `flutter create`。检查并提交生成的平台文件。Android 需要对应 SDK，iOS/macOS 需要 Xcode，签名和发布仍由平台流程显式执行。

```sh
one run platforms -p one-template-flutter-app -- --platforms android,ios
one dev -p one-template-flutter-app -- -d DEVICE_ID
one build -p one-template-flutter-app -- apk
one run doctor -p one-template-flutter-app
```

Keep package metadata in `pubspec.yaml` and tasks in `Taskfile.yml`. After changing tasks or local dependencies, run `one init mise`. Dependencies on packages in this workspace resolve locally by their pub package name.

包信息放在 `pubspec.yaml`，任务放在 `Taskfile.yml`。修改任务或本地依赖后运行 `one init mise`。通过 pub 包名声明的工作区内部依赖会在本地解析。
