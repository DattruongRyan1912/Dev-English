import 'package:flutter/material.dart';

import '../app_controller.dart';
import '../components.dart';
import '../theme.dart';

class ReviewScreen extends StatefulWidget {
  const ReviewScreen({super.key, required this.controller});

  final AppController controller;

  @override
  State<ReviewScreen> createState() => _ReviewScreenState();
}

class _ReviewScreenState extends State<ReviewScreen> {
  int _index = 0;
  bool _showAnswer = false;

  @override
  Widget build(BuildContext context) {
    final items = widget.controller.review;
    if (items.isEmpty) {
      return const PageFrame(
        title: 'Review',
        subtitle: 'Due items first. Try before you reveal the answer.',
        child: EmptyState(
          title: 'You are up to date',
          description:
              'New mistakes and vocabulary will appear here when they are ready.',
        ),
      );
    }
    final item = items[_index.clamp(0, items.length - 1)];
    return PageFrame(
      title: 'Review',
      subtitle: 'Due items first. Try before you reveal the answer.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '${_index + 1} / ${items.length}',
            style: Theme.of(context).textTheme.labelMedium,
          ),
          const SizedBox(height: AppSpacing.md),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(AppSpacing.xl),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 10,
                      vertical: 6,
                    ),
                    decoration: BoxDecoration(
                      color: AppColors.accentSoft,
                      borderRadius: BorderRadius.circular(99),
                    ),
                    child: Text(
                      item.kind,
                      style: Theme.of(context).textTheme.labelMedium?.copyWith(
                        color: AppColors.accent,
                      ),
                    ),
                  ),
                  const SizedBox(height: AppSpacing.xl),
                  Text(
                    item.prompt,
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                  const SizedBox(height: AppSpacing.lg),
                  if (_showAnswer)
                    Container(
                      width: double.infinity,
                      padding: const EdgeInsets.all(AppSpacing.lg),
                      decoration: BoxDecoration(
                        color: AppColors.background,
                        borderRadius: BorderRadius.circular(10),
                      ),
                      child: Text(
                        item.answer,
                        style: Theme.of(context).textTheme.bodyLarge,
                      ),
                    ),
                  const SizedBox(height: AppSpacing.xl),
                  if (!_showAnswer)
                    OutlinedButton(
                      onPressed: () => setState(() => _showAnswer = true),
                      child: const Text('Reveal answer'),
                    )
                  else
                    Row(
                      children: [
                        Expanded(
                          child: OutlinedButton(
                            onPressed: () => _review(item.kind, item.id, false),
                            child: const Text('Again'),
                          ),
                        ),
                        const SizedBox(width: AppSpacing.sm),
                        Expanded(
                          child: FilledButton(
                            onPressed: () => _review(item.kind, item.id, true),
                            child: const Text('Got it'),
                          ),
                        ),
                      ],
                    ),
                ],
              ),
            ),
          ),
          const SizedBox(height: AppSpacing.xl),
          Text('Context', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: AppSpacing.sm),
          Text(item.context, style: Theme.of(context).textTheme.bodyMedium),
        ],
      ),
    );
  }

  Future<void> _review(String kind, String id, bool success) async {
    await widget.controller.submitReview(kind: kind, id: id, success: success);
    if (!mounted) return;
    setState(() {
      _showAnswer = false;
      if (widget.controller.review.isNotEmpty) {
        _index = _index.clamp(0, widget.controller.review.length - 1);
      }
    });
  }
}
