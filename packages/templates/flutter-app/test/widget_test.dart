import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:one_template_flutter_app/main.dart';

void main() {
  testWidgets('increments the counter', (tester) async {
    await tester.pumpWidget(const StarterApp());
    expect(find.text('Button pressed 0 times'), findsOneWidget);
    await tester.tap(find.byType(FilledButton));
    await tester.pump();
    expect(find.text('Button pressed 1 times'), findsOneWidget);
  });
  testWidgets('renders Chinese copy', (tester) async {
    tester.platformDispatcher.localesTestValue = const [Locale('zh', 'CN')];
    addTearDown(tester.platformDispatcher.clearLocalesTestValue);
    await tester.pumpWidget(const StarterApp());
    await tester.pumpAndSettle();
    expect(find.text('Flutter 工作区已就绪。'), findsOneWidget);
    expect(find.text('增加计数'), findsOneWidget);
  });
}
