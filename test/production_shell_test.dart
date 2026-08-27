import 'package:devenglish/main.dart';
import 'package:devenglish/src/api.dart';
import 'package:devenglish/src/app_controller.dart';
import 'package:devenglish/src/demo_data.dart';
import 'package:devenglish/src/models.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

const _isProduction =
    String.fromEnvironment('DEVENGLISH_ENV', defaultValue: 'development') ==
    'production';

class _AuthenticatedApi extends DevEnglishApi {
  _AuthenticatedApi() : super(baseUrl: 'http://127.0.0.1:1');

  @override
  Future<AuthUser> currentUser() async =>
      const AuthUser(id: 'user-1', displayName: 'Ryan', cefr: 'B1');

  @override
  Future<HomeData> home() async => DemoData.home;

  @override
  Future<List<PracticeMode>> practice() async => DemoData.practice;

  @override
  Future<List<ReviewItem>> reviewDue() async => DemoData.review;

  @override
  Future<ProgressData> progress() async => DemoData.progress;

  @override
  Future<DiagnosticResult> diagnosticResult() async =>
      throw StateError('diagnostic fixture is not needed for this shell test');

  @override
  Future<AnalyticsSummary> analytics() async => DemoData.analytics;

  @override
  Future<WeeklySpeakingAssessment> weeklySpeaking() async =>
      DemoData.weeklySpeaking;
}

void main() {
  testWidgets(
    'successful authenticated production bootstrap keeps canonical legacy shell',
    (tester) async {
      final controller = AppController(api: _AuthenticatedApi());
      addTearDown(controller.dispose);

      await tester.pumpWidget(DevEnglishApp(controller: controller));
      await tester.pumpAndSettle();

      expect(controller.authenticated, isTrue);
      expect(controller.usingDemo, isFalse);
      expect(find.text("Today's mission"), findsOneWidget);
      expect(find.text('Ask your assistant'), findsNothing);
      expect(find.textContaining('Local preview:'), findsNothing);

      Future<void> selectDestination(String label) async {
        final destination = find.descendant(
          of: find.byType(NavigationBar),
          matching: find.text(label),
        );
        expect(destination, findsOneWidget);
        await tester.tap(destination);
        await tester.pumpAndSettle();
      }

      await selectDestination('Practice');
      expect(
        find.text('Choose one clear task. Keep the session moving.'),
        findsOneWidget,
      );
      await selectDestination('Review');
      expect(
        find.text('Due items first. Try before you reveal the answer.'),
        findsOneWidget,
      );
      await selectDestination('Progress');
      expect(
        find.text('Trends that help you choose the next useful practice.'),
        findsOneWidget,
      );
      await selectDestination('Home');
      expect(find.text("Today's mission"), findsOneWidget);
    },
    skip: !_isProduction,
  );
}
