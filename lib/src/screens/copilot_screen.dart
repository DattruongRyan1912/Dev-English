import 'package:flutter/material.dart';

import '../app_controller.dart';
import '../theme.dart';

class CopilotScreen extends StatefulWidget {
  const CopilotScreen({super.key, required this.controller});

  final AppController controller;

  @override
  State<CopilotScreen> createState() => _CopilotScreenState();
}

class _CopilotScreenState extends State<CopilotScreen> {
  final _requestController = TextEditingController();
  final _contextController = TextEditingController();

  @override
  void dispose() {
    _requestController.dispose();
    _contextController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.background,
      appBar: AppBar(
        title: const Text('English Copilot'),
        backgroundColor: AppColors.background,
        surfaceTintColor: Colors.transparent,
      ),
      body: SafeArea(
        child: AnimatedBuilder(
          animation: widget.controller,
          builder: (context, _) => ListView(
            padding: const EdgeInsets.fromLTRB(
              AppSpacing.xl,
              AppSpacing.md,
              AppSpacing.xl,
              AppSpacing.section,
            ),
            children: [
              Text(
                'Turn intent into developer English.',
                style: Theme.of(context).textTheme.titleLarge,
              ),
              const SizedBox(height: AppSpacing.sm),
              Text(
                'Keep the meaning, then compare simple, natural and professional wording.',
                style: Theme.of(context).textTheme.bodyMedium,
              ),
              const SizedBox(height: AppSpacing.xl),
              TextField(
                controller: _requestController,
                minLines: 3,
                maxLines: 6,
                decoration: const InputDecoration(
                  labelText: 'Vietnamese intent',
                  alignLabelWithHint: true,
                  hintText: 'Ví dụ: nhờ đồng đội kiểm tra lại timeout của API',
                ),
              ),
              const SizedBox(height: AppSpacing.lg),
              TextField(
                controller: _contextController,
                minLines: 2,
                maxLines: 4,
                decoration: const InputDecoration(
                  labelText: 'Optional technical context',
                  alignLabelWithHint: true,
                ),
              ),
              const SizedBox(height: AppSpacing.xl),
              FilledButton(
                onPressed: widget.controller.working ? null : _run,
                child: Text(
                  widget.controller.working
                      ? 'Writing options…'
                      : 'Generate options',
                ),
              ),
              if (widget.controller.copilotResult != null) ...[
                const SizedBox(height: AppSpacing.section),
                _OptionCard(
                  label: 'Simple',
                  value: widget.controller.copilotResult!.simple,
                ),
                _OptionCard(
                  label: 'Natural',
                  value: widget.controller.copilotResult!.natural,
                ),
                _OptionCard(
                  label: 'Professional',
                  value: widget.controller.copilotResult!.professional,
                ),
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(AppSpacing.lg),
                    child: Text(
                      widget.controller.copilotResult!.explanation,
                      style: Theme.of(context).textTheme.bodyMedium,
                    ),
                  ),
                ),
              ],
              if (widget.controller.error != null) ...[
                const SizedBox(height: AppSpacing.lg),
                Text(
                  widget.controller.error!,
                  style: Theme.of(
                    context,
                  ).textTheme.bodyMedium?.copyWith(color: AppColors.error),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _run() => widget.controller.runCopilot(
    _requestController.text,
    context: _contextController.text,
  );
}

class _OptionCard extends StatelessWidget {
  const _OptionCard({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) => Card(
    child: Padding(
      padding: const EdgeInsets.all(AppSpacing.lg),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: Theme.of(context).textTheme.labelMedium),
          const SizedBox(height: AppSpacing.sm),
          SelectableText(value, style: Theme.of(context).textTheme.bodyLarge),
        ],
      ),
    ),
  );
}
