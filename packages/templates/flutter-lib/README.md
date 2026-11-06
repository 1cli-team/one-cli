# one-template-flutter-lib

Shared Flutter widgets, resolved from the root pub workspace. Import `widgets.dart` from this package in an application. Libraries are consumed as Dart source; they have analyze, test and format tasks, and no artificial application build step.

共享 Flutter 组件库，通过根目录 pub workspace 解析。在应用中导入本包的 `widgets.dart`。组件库以 Dart 源码供应用使用，提供分析、测试和格式化任务，不添加应用构建步骤。

```sh
one test -p one-template-flutter-lib
one run check -p one-template-flutter-lib
one run format -p one-template-flutter-lib
```

Add this package's pub name to an app's `dependencies` in `pubspec.yaml`, then run `one init mise` to refresh the task dependency graph. The next development run resolves the entire pub workspace. Commit the root `pubspec.lock`.

在应用 `pubspec.yaml` 的 `dependencies` 中添加本包的 pub 包名，然后运行 `one init mise` 刷新任务依赖图。下次开发运行会解析整个 pub 工作区。请提交根目录 `pubspec.lock`。
