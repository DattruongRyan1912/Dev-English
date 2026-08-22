import 'package:devenglish/src/app_controller.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('runtime flags keep production authenticated and fail-closed', () {
    final controller = AppController();
    addTearDown(controller.dispose);
    const production =
        String.fromEnvironment('DEVENGLISH_ENV', defaultValue: 'development') ==
        'production';

    expect(controller.requiresAuthentication, production);
    expect(controller.demoFallbackEnabled, !production);
    expect(controller.authenticated, !production);
  });
}
