import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:one_template_flutter_lib/widgets.dart';

void main() {
  testWidgets('greets the supplied name', (tester) async {
    await tester.pumpWidget(const MaterialApp(home: Greeting(name: 'One')));
    expect(find.text('Hello, One!'), findsOneWidget);
  });
}
