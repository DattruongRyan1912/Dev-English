import 'package:flutter/material.dart';

import '../app_controller.dart';
import '../components.dart';
import '../models.dart';
import '../theme.dart';

class ProgressScreen extends StatelessWidget {
  const ProgressScreen({super.key, required this.controller});

  final AppController controller;

  @override
  Widget build(BuildContext context) {
    final progress = controller.progress;
    return PageFrame(
      title: 'Progress',
      subtitle: 'Trends that help you choose the next useful practice.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Card(
            child: Padding(
              padding: const EdgeInsets.all(AppSpacing.xl),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Skill progression',
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  Text(
                    'Your score reflects competency, not vocabulary count.',
                    style: Theme.of(context).textTheme.bodyMedium,
                  ),
                  ...progress.state.skills.map(
                    (skill) => SkillBar(skill: skill),
                  ),
                ],
              ),
            ),
          ),
          const SizedBox(height: AppSpacing.section),
          const SectionTitle('This week'),
          const SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Expanded(
                child: _Metric(
                  label: 'Technical score',
                  value: '${progress.technicalScore.round()}',
                ),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: _Metric(
                  label: 'Repeated mistakes',
                  value: '${progress.repeatedMistakes}',
                ),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: _Metric(
                  label: 'VN fallback',
                  value: '${progress.fallback.round()}%',
                ),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.section),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(AppSpacing.xl),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Learning analytics',
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                  const SizedBox(height: AppSpacing.md),
                  Wrap(
                    spacing: AppSpacing.md,
                    runSpacing: AppSpacing.md,
                    children: [
                      _InlineMetric(
                        label: 'Minutes learned',
                        value: '${controller.analytics.minutesLearned}',
                      ),
                      _InlineMetric(
                        label: 'Missions completed',
                        value: '${controller.analytics.missionsCompleted}',
                      ),
                      _InlineMetric(
                        label: 'Vocabulary mastery',
                        value:
                            '${(controller.analytics.vocabularyMastery * 100).round()}%',
                      ),
                    ],
                  ),
                  const SizedBox(height: AppSpacing.lg),
                  Text(
                    'Weekly speaking · ${controller.weeklySpeaking.sessions} session(s) · ${controller.weeklySpeaking.evaluated} assessed',
                    style: Theme.of(context).textTheme.bodyMedium,
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  Text(
                    controller.weeklySpeaking.recommendation,
                    style: Theme.of(context).textTheme.bodyLarge,
                  ),
                ],
              ),
            ),
          ),
          const SizedBox(height: AppSpacing.section),
          const SectionTitle('Score trend'),
          const SizedBox(height: AppSpacing.md),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(AppSpacing.xl),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Developer English score',
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                  const SizedBox(height: AppSpacing.xl),
                  SizedBox(
                    height: 120,
                    child: _TrendChart(points: progress.trend),
                  ),
                  const SizedBox(height: AppSpacing.md),
                  Text(
                    'Your score is moving up steadily. Keep the daily mission small and specific.',
                    style: Theme.of(context).textTheme.bodyMedium,
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _Metric extends StatelessWidget {
  const _Metric({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              value,
              style: Theme.of(
                context,
              ).textTheme.titleLarge?.copyWith(color: AppColors.accent),
            ),
            const SizedBox(height: AppSpacing.xs),
            Text(label, style: Theme.of(context).textTheme.labelMedium),
          ],
        ),
      ),
    );
  }
}

class _InlineMetric extends StatelessWidget {
  const _InlineMetric({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(
      horizontal: AppSpacing.md,
      vertical: AppSpacing.sm,
    ),
    decoration: BoxDecoration(
      color: AppColors.accentSoft,
      borderRadius: BorderRadius.circular(8),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(value, style: Theme.of(context).textTheme.titleMedium),
        Text(label, style: Theme.of(context).textTheme.labelMedium),
      ],
    ),
  );
}

class _TrendChart extends StatelessWidget {
  const _TrendChart({required this.points});

  final List<ProgressPoint> points;

  @override
  Widget build(BuildContext context) => CustomPaint(
    painter: _TrendPainter(points),
    child: const SizedBox.expand(),
  );
}

class _TrendPainter extends CustomPainter {
  _TrendPainter(this.points);

  final List<ProgressPoint> points;

  @override
  void paint(Canvas canvas, Size size) {
    if (points.isEmpty) return;
    final line = Paint()
      ..color = AppColors.accent
      ..strokeWidth = 3
      ..style = PaintingStyle.stroke
      ..strokeCap = StrokeCap.round;
    final dot = Paint()
      ..color = AppColors.accent
      ..style = PaintingStyle.fill;
    final scores = points.map((point) => point.score).toList();
    final minScore = scores.reduce((a, b) => a < b ? a : b) - 4;
    final maxScore = scores.reduce((a, b) => a > b ? a : b) + 4;
    final path = Path();
    for (var i = 0; i < points.length; i++) {
      final x = points.length == 1
          ? size.width / 2
          : i * size.width / (points.length - 1);
      final y =
          size.height -
          ((points[i].score - minScore) / (maxScore - minScore)) * size.height;
      if (i == 0) {
        path.moveTo(x, y);
      } else {
        path.lineTo(x, y);
      }
      canvas.drawCircle(Offset(x, y), 4, dot);
    }
    canvas.drawPath(path, line);
  }

  @override
  bool shouldRepaint(covariant _TrendPainter oldDelegate) =>
      oldDelegate.points != points;
}
