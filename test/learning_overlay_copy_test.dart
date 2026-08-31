import 'package:devenglish/src/learning_overlay_copy.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('keeps the current B1 overlay copy compatible', () {
    final copy = LearningOverlayCopy.forLevel('B1');

    expect(copy.level, 'B1');
    expect(copy.label, 'practical');
    expect(copy.workflowStarter, contains('My next step is to'));
    expect(copy.followUp, contains('điền một chi tiết'));
  });

  test('selects simpler guided language for beginner levels', () {
    final copy = LearningOverlayCopy.forLevel('a1');

    expect(copy.level, 'A1');
    expect(copy.label, 'guided');
    expect(copy.supportStarter, 'The problem is ____. I will check ____.');
    expect(copy.followUp, 'What will you check first?');
  });

  test('selects evidence-oriented language for advanced levels', () {
    final copy = LearningOverlayCopy.forLevel('C1');

    expect(copy.level, 'C1');
    expect(copy.label, 'precise');
    expect(copy.supportStarter, contains('validate'));
    expect(copy.followUp, contains('evidence'));
  });

  test('unknown profile levels fall back to a safe practical overlay', () {
    final copy = LearningOverlayCopy.forLevel('unknown');

    expect(copy.level, 'B1');
    expect(copy.label, 'practical');
  });
}
