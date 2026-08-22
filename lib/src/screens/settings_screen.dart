import 'package:flutter/material.dart';

import '../app_controller.dart';
import '../models.dart';
import '../theme.dart';

class SettingsScreen extends StatefulWidget {
  const SettingsScreen({super.key, required this.controller});

  final AppController controller;

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  @override
  void initState() {
    super.initState();
    widget.controller.loadSettingsData();
  }

  @override
  Widget build(BuildContext context) => Scaffold(
    backgroundColor: AppColors.background,
    appBar: AppBar(
      title: const Text('Settings'),
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
            Card(
              child: ListTile(
                contentPadding: const EdgeInsets.all(AppSpacing.lg),
                leading: const Icon(
                  Icons.auto_awesome_outlined,
                  color: AppColors.accent,
                ),
                title: const Text('AI provider'),
                subtitle: Text(
                  widget.controller.home.provider.configured
                      ? widget.controller.home.provider.name
                      : 'Deterministic fallback · configure keys on backend',
                ),
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            Card(
              child: Column(
                children: [
                  ListTile(
                    contentPadding: const EdgeInsets.all(AppSpacing.lg),
                    leading: const Icon(Icons.tune_outlined),
                    title: const Text('Models & pronunciation'),
                    subtitle: Text(
                      'Fast: ${widget.controller.settings.fastModel}\nSmart: ${widget.controller.settings.smartModel}\nBudget: ${widget.controller.settings.monthlyBudgetVnd} VND / month',
                    ),
                  ),
                  SwitchListTile.adaptive(
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: AppSpacing.lg,
                    ),
                    title: const Text('Pronunciation assessment'),
                    subtitle: const Text(
                      'Use Azure pronunciation and prosody when configured.',
                    ),
                    value: widget.controller.settings.pronunciationOn,
                    onChanged: widget.controller.updatePronunciation,
                  ),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            Card(
              child: Padding(
                padding: const EdgeInsets.all(AppSpacing.lg),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            'Provider connections',
                            style: Theme.of(context).textTheme.titleMedium,
                          ),
                        ),
                        IconButton(
                          onPressed: widget.controller.testConnections,
                          tooltip: 'Test connections',
                          icon: const Icon(Icons.refresh),
                        ),
                      ],
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    ...widget.controller.providerChecks.map(
                      (check) => _ProviderRow(check: check),
                    ),
                    const SizedBox(height: AppSpacing.md),
                    SizedBox(
                      width: double.infinity,
                      child: OutlinedButton.icon(
                        onPressed: widget.controller.testConnections,
                        icon: const Icon(Icons.network_check),
                        label: const Text('Test connection status'),
                      ),
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            Card(
              child: ListTile(
                contentPadding: const EdgeInsets.all(AppSpacing.lg),
                leading: const Icon(Icons.schedule_outlined),
                title: const Text('Daily routine'),
                subtitle: const Text(
                  '45–60 minutes · mission, review and one retry',
                ),
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            Card(
              child: ListTile(
                contentPadding: const EdgeInsets.all(AppSpacing.lg),
                leading: const Icon(Icons.lock_outline),
                title: const Text('Privacy'),
                subtitle: const Text(
                  'API keys stay on the backend. Pasted work context should not contain secrets.',
                ),
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            if (widget.controller.usageSummary != null)
              Card(
                child: ListTile(
                  contentPadding: const EdgeInsets.all(AppSpacing.lg),
                  leading: const Icon(Icons.payments_outlined),
                  title: const Text('AI usage'),
                  subtitle: Text(
                    '${widget.controller.usageSummary!.estimatedCost.toStringAsFixed(0)} VND estimated this month · ${widget.controller.usageSummary!.budgetUsedPercent.toStringAsFixed(1)}% of budget',
                  ),
                ),
              ),
            if (widget.controller.error != null) ...[
              const SizedBox(height: AppSpacing.lg),
              Text(
                widget.controller.error!,
                style: Theme.of(
                  context,
                ).textTheme.bodyMedium?.copyWith(color: AppColors.error),
              ),
            ],
            const SizedBox(height: AppSpacing.xl),
            Text(
              'Settings change models and budgets; they never expose provider secrets in the client.',
              style: Theme.of(context).textTheme.bodyMedium,
            ),
          ],
        ),
      ),
    ),
  );
}

class _ProviderRow extends StatelessWidget {
  const _ProviderRow({required this.check});

  final ProviderCheck check;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(vertical: AppSpacing.xs),
    child: Row(
      children: [
        Icon(
          check.configured ? Icons.check_circle : Icons.circle_outlined,
          size: 18,
          color: check.configured ? AppColors.success : AppColors.textSecondary,
        ),
        const SizedBox(width: AppSpacing.sm),
        Expanded(child: Text(check.provider)),
        Text(
          check.status == 'fallback'
              ? 'Fallback'
              : check.configured
              ? 'Ready'
              : 'Not configured',
          style: Theme.of(context).textTheme.labelMedium,
        ),
      ],
    ),
  );
}
