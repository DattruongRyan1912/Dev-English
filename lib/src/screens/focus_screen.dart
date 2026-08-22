import 'package:flutter/material.dart';

import '../app_controller.dart';
import '../components.dart';
import '../models.dart';
import '../theme.dart';

class FocusScreen extends StatefulWidget {
  const FocusScreen({super.key, required this.controller});

  final AppController controller;

  @override
  State<FocusScreen> createState() => _FocusScreenState();
}

class _FocusScreenState extends State<FocusScreen> {
  final _answerController = TextEditingController();

  @override
  void dispose() {
    _answerController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.background,
      body: SafeArea(
        child: AnimatedBuilder(
          animation: widget.controller,
          builder: (context, _) {
            final mission = widget.controller.home.mission;
            final submission = widget.controller.lastSubmission;
            final missionComplete =
                submission != null && _isMissionComplete(submission);
            return Padding(
              padding: const EdgeInsets.fromLTRB(
                AppSpacing.lg,
                AppSpacing.lg,
                AppSpacing.lg,
                AppSpacing.xl,
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  FocusHeader(title: mission.title, onExit: _exitMission),
                  const SizedBox(height: AppSpacing.xl),
                  Expanded(
                    child: SingleChildScrollView(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          _ProgressLine(submitted: submission != null),
                          const SizedBox(height: AppSpacing.xl),
                          Text(
                            'Context',
                            style: Theme.of(context).textTheme.labelMedium,
                          ),
                          const SizedBox(height: AppSpacing.sm),
                          Text(
                            mission.context,
                            style: Theme.of(context).textTheme.bodyLarge,
                          ),
                          const SizedBox(height: AppSpacing.xl),
                          Text(
                            'Your task',
                            style: Theme.of(context).textTheme.labelMedium,
                          ),
                          const SizedBox(height: AppSpacing.sm),
                          Text(
                            mission.prompt,
                            style: Theme.of(context).textTheme.titleLarge,
                          ),
                          const SizedBox(height: AppSpacing.xl),
                          TextField(
                            controller: _answerController,
                            minLines: 8,
                            maxLines: 14,
                            textCapitalization: TextCapitalization.sentences,
                            decoration: const InputDecoration(
                              labelText: 'Your answer',
                              alignLabelWithHint: true,
                              hintText:
                                  'Write as if you were sending this to your team...',
                            ),
                          ),
                          if (submission != null) ...[
                            const SizedBox(height: AppSpacing.xl),
                            _Feedback(
                              result: submission,
                              missionComplete: missionComplete,
                            ),
                          ],
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(height: AppSpacing.lg),
                  SizedBox(
                    width: double.infinity,
                    child: FilledButton(
                      onPressed:
                          widget.controller.submitting ||
                              widget.controller.loading
                          ? null
                          : missionComplete
                          ? _continueToNextMission
                          : () => widget.controller.submitWriting(
                              mission.id,
                              _answerController.text,
                            ),
                      child: Text(
                        widget.controller.submitting
                            ? 'Reviewing your explanation…'
                            : missionComplete
                            ? 'Continue to next mission'
                            : submission == null
                            ? 'Check answer'
                            : 'Check again',
                      ),
                    ),
                  ),
                ],
              ),
            );
          },
        ),
      ),
    );
  }

  bool _isMissionComplete(SubmissionResult submission) =>
      submission.evaluation.score >= 70 && submission.mistakeCount <= 1;

  void _exitMission() {
    Navigator.of(context).pop();
    widget.controller.refreshAfterMissionExit();
  }

  Future<void> _continueToNextMission() async {
    _answerController.clear();
    await widget.controller.continueToNextMission();
  }
}

class _ProgressLine extends StatelessWidget {
  const _ProgressLine({required this.submitted});

  final bool submitted;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: ClipRRect(
            borderRadius: BorderRadius.circular(4),
            child: LinearProgressIndicator(
              value: submitted ? 1 : .5,
              minHeight: 6,
              color: AppColors.accent,
              backgroundColor: AppColors.border,
            ),
          ),
        ),
        const SizedBox(width: AppSpacing.md),
        Text(
          submitted ? '2 / 2' : '1 / 2',
          style: Theme.of(context).textTheme.labelMedium,
        ),
      ],
    );
  }
}

class _Feedback extends StatelessWidget {
  const _Feedback({required this.result, required this.missionComplete});

  final SubmissionResult result;
  final bool missionComplete;

  @override
  Widget build(BuildContext context) {
    final evaluation = result.evaluation;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppSpacing.xl),
      decoration: BoxDecoration(
        color: AppColors.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  'Feedback',
                  style: Theme.of(context).textTheme.titleLarge,
                ),
              ),
              Text(
                '${evaluation.score.round()} / 100',
                style: Theme.of(
                  context,
                ).textTheme.titleMedium?.copyWith(color: AppColors.accent),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.lg),
          Text(
            evaluation.summary,
            style: Theme.of(context).textTheme.bodyLarge,
          ),
          const SizedBox(height: AppSpacing.lg),
          Text('What was good', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: AppSpacing.sm),
          ...evaluation.good.map((item) => _Bullet(text: item)),
          const SizedBox(height: AppSpacing.lg),
          Text('Main issue', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: AppSpacing.sm),
          Text(
            evaluation.mainIssue,
            style: Theme.of(context).textTheme.bodyLarge,
          ),
          if (evaluation.corrections.isNotEmpty) ...[
            const SizedBox(height: AppSpacing.lg),
            Text(
              'Try this correction',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: AppSpacing.sm),
            ...evaluation.corrections.map(
              (item) => _CorrectionCard(item: item),
            ),
          ],
          const SizedBox(height: AppSpacing.lg),
          Text(
            missionComplete
                ? 'Next action: Continue to the next mission.'
                : 'Next action: ${evaluation.nextAction}',
            style: Theme.of(
              context,
            ).textTheme.bodyLarge?.copyWith(fontWeight: FontWeight.w600),
          ),
        ],
      ),
    );
  }
}

class _CorrectionCard extends StatelessWidget {
  const _CorrectionCard({required this.item});

  final Correction item;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: AppSpacing.sm),
      padding: const EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: AppColors.background,
        borderRadius: BorderRadius.circular(10),
      ),
      child: RichText(
        text: TextSpan(
          style: Theme.of(context).textTheme.bodyMedium,
          children: [
            TextSpan(
              text: '${item.original}  →  ',
              style: const TextStyle(
                color: AppColors.error,
                fontWeight: FontWeight.w600,
              ),
            ),
            TextSpan(
              text: item.corrected,
              style: const TextStyle(
                color: AppColors.success,
                fontWeight: FontWeight.w600,
              ),
            ),
            TextSpan(text: '\n${item.why}'),
          ],
        ),
      ),
    );
  }
}

class _Bullet extends StatelessWidget {
  const _Bullet({required this.text});

  final String text;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Padding(
            padding: EdgeInsets.only(top: 5),
            child: Icon(Icons.check, size: 16, color: AppColors.success),
          ),
          const SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Text(text, style: Theme.of(context).textTheme.bodyLarge),
          ),
        ],
      ),
    );
  }
}
