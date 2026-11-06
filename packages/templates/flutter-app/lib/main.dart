import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';

void main() => runApp(const StarterApp());

class StarterApp extends StatelessWidget {
  const StarterApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'One Flutter',
      localizationsDelegates: GlobalMaterialLocalizations.delegates,
      supportedLocales: const [Locale('en', 'US'), Locale('zh', 'CN')],
      theme: ThemeData(colorSchemeSeed: Colors.indigo, useMaterial3: true),
      home: const HomePage(),
    );
  }
}

class HomePage extends StatefulWidget {
  const HomePage({super.key});

  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  int _count = 0;

  @override
  Widget build(BuildContext context) {
    final chinese = Localizations.localeOf(context).languageCode == 'zh';
    return Scaffold(
      appBar: AppBar(title: const Text('One Flutter')),
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              chinese ? 'Flutter 工作区已就绪。' : 'Your Flutter workspace is ready.',
            ),
            const SizedBox(height: 16),
            Text(chinese ? '已点击 $_count 次' : 'Button pressed $_count times'),
            const SizedBox(height: 16),
            FilledButton(
              onPressed: () => setState(() => _count++),
              child: Text(chinese ? '增加计数' : 'Increment'),
            ),
          ],
        ),
      ),
    );
  }
}
