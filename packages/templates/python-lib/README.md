# one-template-python-lib

Python library managed by the root uv workspace. One synchronizes all members into the shared `.venv` before running tasks.

由根目录 uv 工作区管理的 Python 共享库。One 在执行任务前将所有成员同步到共享 `.venv`。

```sh
one run check -p one-template-python-lib
one run build -p one-template-python-lib
```

Add internal packages to `project.dependencies` and mark them with `workspace = true` in `tool.uv.sources`. Run `one init mise` after changing tasks or internal dependencies.

在 `project.dependencies` 中声明内部包，并在 `tool.uv.sources` 中设置 `workspace = true`。修改任务或内部依赖后执行 `one init mise`。

The import module is `one_template_python_lib`.

导入模块名为 `one_template_python_lib`。

Declare One tasks in `[tool.one.tasks.<name>]` in `pyproject.toml`, with `run` as an argument array. One also discovers `[project.scripts]` entry points and runs them through uv.

在 `pyproject.toml` 的 `[tool.one.tasks.<name>]` 中声明任务，`run` 使用参数数组。One 同时发现 `[project.scripts]` 命令入口，并通过 uv 执行。
