import 'package:devenglish/main.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders the focus-first home shell', (tester) async {
    await tester.pumpWidget(const DevEnglishApp());
    await tester.pump();
    expect(find.text('Today\'s mission'), findsOneWidget);
    expect(find.text('Start mission'), findsOneWidget);
    expect(find.text('Practice'), findsOneWidget);
  });
}
