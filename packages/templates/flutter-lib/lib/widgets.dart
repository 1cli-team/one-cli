import 'package:flutter/widgets.dart';

class Greeting extends StatelessWidget {
  const Greeting({super.key, required this.name});

  final String name;

  @override
  Widget build(BuildContext context) {
    final chinese = Localizations.localeOf(context).languageCode == 'zh';
    return Text(chinese ? '你好，$name！' : 'Hello, $name!');
  }
}
