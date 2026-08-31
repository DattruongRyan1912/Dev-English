import 'package:flutter/material.dart';

import '../app_controller.dart';
import '../models.dart';
import '../theme.dart';

class SettingsScreen extends StatefulWidget {
  const SettingsScreen({super.key, required this.controller, this.onSignedOut});

  final AppController controller;
  final VoidCallback? onSignedOut;

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  final _deepSeekKeyController = TextEditingController();
  final _fastModelController = TextEditingController();
  final _smartModelController = TextEditingController();

  @override
  void initState() {
    super.initState();
    widget.controller.loadSettingsData().then((_) {
      if (!mounted) return;
      _fastModelController.text = widget.controller.settings.fastModel;
      _smartModelController.text = widget.controller.settings.smartModel;
      setState(() {});
    });
  }

  @override
  void dispose() {
    _deepSeekKeyController.dispose();
    _fastModelController.dispose();
    _smartModelController.dispose();
    super.dispose();
  }

  Future<void> _signOut() async {
    await widget.controller.logout();
    widget.onSignedOut?.call();
    if (!mounted) return;
    Navigator.of(context).pop();
  }

  Future<void> _saveDeepSeekKey() async {
    final key = _deepSeekKeyController.text.trim();
    if (key.isEmpty) return;
    await widget.controller.setDeepSeekKey(key);
    _deepSeekKeyController.clear();
  }

  Future<void> _saveModels() => widget.controller.saveModelSettings(
    fastModel: _fastModelController.text,
    smartModel: _smartModelController.text,
  );

  String _statusLabel(String status) => switch (status) {
    'connected' => 'Connected',
    'invalid' => 'Invalid',
    _ => 'Not configured',
  };

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
              child: Padding(
                padding: const EdgeInsets.all(AppSpacing.lg),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'DeepSeek API key',
                      style: Theme.of(context).textTheme.titleMedium,
                    ),
                    const SizedBox(height: AppSpacing.xs),
                    Text(
                      'Status: ${_statusLabel(widget.controller.settings.deepSeekStatus)}. The key is encrypted and retained only by the backend.',
                    ),
                    const SizedBox(height: AppSpacing.md),
                    TextField(
                      controller: _deepSeekKeyController,
                      obscureText: true,
                      autocorrect: false,
                      enableSuggestions: false,
                      decoration: const InputDecoration(
                        labelText: 'New key / replacement key',
                        hintText: 'Paste only over HTTPS or local development',
                      ),
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    Wrap(
                      spacing: AppSpacing.sm,
                      runSpacing: AppSpacing.sm,
                      children: [
                        FilledButton.icon(
                          onPressed: _saveDeepSeekKey,
                          icon: const Icon(Icons.save_outlined),
                          label: Text(
                            widget.controller.settings.deepSeekConfigured
                                ? 'Replace'
                                : 'Set',
                          ),
                        ),
                        OutlinedButton.icon(
                          onPressed: widget.controller.testDeepSeekKey,
                          icon: const Icon(Icons.network_check),
                          label: const Text('Test'),
                        ),
                        OutlinedButton.icon(
                          onPressed:
                              widget.controller.settings.deepSeekConfigured
                              ? widget.controller.removeDeepSeekKey
                              : null,
                          icon: const Icon(Icons.delete_outline),
                          label: const Text('Remove'),
                        ),
                      ],
                    ),
                  ],
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
                  Padding(
                    padding: const EdgeInsets.symmetric(
                      horizontal: AppSpacing.lg,
                    ),
                    child: Column(
                      children: [
                        TextField(
                          controller: _fastModelController,
                          decoration: const InputDecoration(
                            labelText: 'Fast model',
                          ),
                        ),
                        const SizedBox(height: AppSpacing.sm),
                        TextField(
                          controller: _smartModelController,
                          decoration: const InputDecoration(
                            labelText: 'Smart model',
                          ),
                        ),
                        const SizedBox(height: AppSpacing.sm),
                        Align(
                          alignment: Alignment.centerRight,
                          child: OutlinedButton(
                            onPressed: _saveModels,
                            child: const Text('Save models'),
                          ),
                        ),
                      ],
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
            if (widget.controller.requiresAuthentication) ...[
              const SizedBox(height: AppSpacing.md),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(AppSpacing.lg),
                  child: OutlinedButton.icon(
                    onPressed: widget.controller.authenticating
                        ? null
                        : _signOut,
                    icon: const Icon(Icons.logout),
                    label: const Text('Sign out'),
                  ),
                ),
              ),
            ],
            const SizedBox(height: AppSpacing.md),
            if (widget.controller.usageSummary != null)
              Card(
                child: ListTile(
                  contentPadding: const EdgeInsets.all(AppSpacing.lg),
                  leading: const Icon(Icons.payments_outlined),
                  title: const Text('AI usage'),
                  subtitle: Builder(
                    builder: (context) {
                      final usage = widget.controller.usageSummary!;
                      return Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            '${usage.estimatedCost.toStringAsFixed(0)} VND estimated this month · ${usage.budgetUsedPercent.toStringAsFixed(1)}% of budget',
                          ),
                          if (usage.unavailableRecords > 0) ...[
                            const SizedBox(height: AppSpacing.xs),
                            Text(
                              '${usage.unavailableRecords} request(s) have no provider usage data; they are excluded from this cost total.',
                              style: Theme.of(context).textTheme.bodySmall,
                            ),
                          ],
                        ],
                      );
                    },
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
          check.status == 'healthy'
              ? Icons.check_circle
              : check.status == 'unhealthy'
              ? Icons.error_outline
              : Icons.circle_outlined,
          size: 18,
          color: check.status == 'healthy'
              ? AppColors.success
              : check.status == 'unhealthy'
              ? AppColors.error
              : AppColors.textSecondary,
        ),
        const SizedBox(width: AppSpacing.sm),
        Expanded(child: Text(check.provider)),
        Text(switch (check.status) {
          'healthy' => 'Healthy',
          'unhealthy' => 'Unhealthy',
          'fallback' => 'Fallback',
          _ => 'Not configured',
        }, style: Theme.of(context).textTheme.labelMedium),
      ],
    ),
  );
}
