import 'package:flutter/material.dart';

import '../app_controller.dart';
import '../models.dart';
import '../theme.dart';

class DiagnosticScreen extends StatefulWidget {
  const DiagnosticScreen({super.key, required this.controller});

  final AppController controller;

  @override
  State<DiagnosticScreen> createState() => _DiagnosticScreenState();
}

class _DiagnosticScreenState extends State<DiagnosticScreen> {
  final Map<String, String> _answers = <String, String>{};
  int _index = 0;

  @override
  void initState() {
    super.initState();
    widget.controller.loadDiagnostic();
  }

  @override
  Widget build(BuildContext context) => Scaffold(
    backgroundColor: AppColors.background,
    appBar: AppBar(
      title: const Text('Diagnostic'),
      backgroundColor: AppColors.background,
      surfaceTintColor: Colors.transparent,
    ),
    body: SafeArea(
      child: AnimatedBuilder(
        animation: widget.controller,
        builder: (context, _) {
          final questions = widget.controller.diagnosticQuestions;
          final result = widget.controller.diagnosticResult;
          if (result != null) {
            return _ResultView(result: result);
          }
          if (questions.isEmpty) {
            final loading = widget.controller.diagnosticLoading;
            return Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (loading) ...[
                    const CircularProgressIndicator(),
                    const SizedBox(height: AppSpacing.lg),
                    const Text('Loading diagnostic...'),
                  ] else ...[
                    const Text(
                      'Diagnostic is temporarily unavailable.',
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    Text(
                      widget.controller.diagnosticError ??
                          'The backend did not return any questions.',
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: AppSpacing.lg),
                    FilledButton.icon(
                      onPressed: widget.controller.loadDiagnostic,
                      icon: const Icon(Icons.refresh),
                      label: const Text('Retry diagnostic'),
                    ),
                  ],
                ],
              ),
            );
          }
          final question = questions[_index.clamp(0, questions.length - 1)];
          return ListView(
            padding: const EdgeInsets.fromLTRB(
              AppSpacing.xl,
              AppSpacing.md,
              AppSpacing.xl,
              AppSpacing.section,
            ),
            children: [
              Text(
                '${_index + 1} / ${questions.length}',
                style: Theme.of(context).textTheme.labelMedium,
              ),
              const SizedBox(height: AppSpacing.md),
              LinearProgressIndicator(
                value: (_index + 1) / questions.length,
                minHeight: 6,
              ),
              const SizedBox(height: AppSpacing.section),
              Text(
                question.prompt,
                style: Theme.of(context).textTheme.titleLarge,
              ),
              const SizedBox(height: AppSpacing.xl),
              ...question.options.map(
                (option) => Padding(
                  padding: const EdgeInsets.only(bottom: AppSpacing.md),
                  child: OutlinedButton(
                    onPressed: () =>
                        _answer(question, option, questions.length),
                    style: OutlinedButton.styleFrom(
                      alignment: Alignment.centerLeft,
                      padding: const EdgeInsets.all(AppSpacing.lg),
                    ),
                    child: Text(option),
                  ),
                ),
              ),
              if (widget.controller.error != null)
                Text(
                  widget.controller.error!,
                  style: Theme.of(
                    context,
                  ).textTheme.bodyMedium?.copyWith(color: AppColors.error),
                ),
            ],
          );
        },
      ),
    ),
  );

  Future<void> _answer(
    DiagnosticQuestion question,
    String answer,
    int total,
  ) async {
    _answers[question.id] = answer;
    if (_index + 1 < total) {
      setState(() => _index++);
      return;
    }
    await widget.controller.submitDiagnostic(_answers);
  }
}

class _ResultView extends StatelessWidget {
  const _ResultView({required this.result});

  final DiagnosticResult result;

  @override
  Widget build(BuildContext context) => ListView(
    padding: const EdgeInsets.fromLTRB(
      AppSpacing.xl,
      AppSpacing.section,
      AppSpacing.xl,
      AppSpacing.section,
    ),
    children: [
      Text(
        'Your starting point',
        style: Theme.of(context).textTheme.headlineSmall,
      ),
      const SizedBox(height: AppSpacing.sm),
      Text(
        '${result.cefr} · ${result.overallScore.round()} / 100',
        style: Theme.of(
          context,
        ).textTheme.titleLarge?.copyWith(color: AppColors.accent),
      ),
      const SizedBox(height: AppSpacing.section),
      Text('Strengths', style: Theme.of(context).textTheme.titleMedium),
      const SizedBox(height: AppSpacing.sm),
      ...result.strengths.map(
        (item) => Text('• $item', style: Theme.of(context).textTheme.bodyLarge),
      ),
      const SizedBox(height: AppSpacing.lg),
      Text('Priorities', style: Theme.of(context).textTheme.titleMedium),
      const SizedBox(height: AppSpacing.sm),
      ...result.priorities.map(
        (item) => Text('• $item', style: Theme.of(context).textTheme.bodyLarge),
      ),
      const SizedBox(height: AppSpacing.lg),
      Text('Recommended plan', style: Theme.of(context).textTheme.titleMedium),
      const SizedBox(height: AppSpacing.sm),
      ...result.recommendedPlan.map(
        (item) => Text('• $item', style: Theme.of(context).textTheme.bodyLarge),
      ),
    ],
  );
}
