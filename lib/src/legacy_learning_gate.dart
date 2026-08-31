import 'package:flutter/material.dart';

import 'app_controller.dart';
import 'theme.dart';

/// Prevents compatibility learning screens from rendering their initial demo
/// state before the backend has supplied real data.
class LegacyLearningGate extends StatefulWidget {
  const LegacyLearningGate({
    super.key,
    required this.controller,
    required this.title,
    required this.builder,
  });

  final AppController controller;
  final String title;
  final WidgetBuilder builder;

  @override
  State<LegacyLearningGate> createState() => _LegacyLearningGateState();
}

class _LegacyLearningGateState extends State<LegacyLearningGate> {
  bool _loading = true;
  bool _available = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _load();
    });
  }

  Future<void> _load() async {
    if (mounted) setState(() => _loading = true);
    final available = await widget.controller.ensureLegacyLearningLoaded();
    if (!mounted) return;
    setState(() {
      _loading = false;
      _available = available;
    });
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return Scaffold(
        key: const ValueKey('legacy-learning-loading'),
        backgroundColor: AppColors.background,
        appBar: AppBar(
          title: Text(widget.title),
          backgroundColor: AppColors.background,
          surfaceTintColor: Colors.transparent,
        ),
        body: const Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              CircularProgressIndicator(),
              SizedBox(height: AppSpacing.lg),
              Text('Loading Learning data...'),
            ],
          ),
        ),
      );
    }
    if (!_available) {
      return Scaffold(
        key: const ValueKey('legacy-learning-unavailable'),
        backgroundColor: AppColors.background,
        appBar: AppBar(
          title: Text(widget.title),
          backgroundColor: AppColors.background,
          surfaceTintColor: Colors.transparent,
        ),
        body: Center(
          child: Padding(
            padding: const EdgeInsets.all(AppSpacing.xl),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.cloud_off_outlined, size: 48),
                const SizedBox(height: AppSpacing.lg),
                const Text(
                  'Learning data is temporarily unavailable.',
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: AppSpacing.sm),
                Text(
                  widget.controller.legacyError ??
                      'The backend did not return the required data.',
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: AppSpacing.xl),
                FilledButton.icon(
                  onPressed: _load,
                  icon: const Icon(Icons.refresh),
                  label: const Text('Retry learning data'),
                ),
              ],
            ),
          ),
        ),
      );
    }
    return widget.builder(context);
  }
}

/// Keeps the canonical shell from showing demo provider settings while the
/// settings endpoint is loading or unavailable.
class SettingsDataGate extends StatefulWidget {
  const SettingsDataGate({
    super.key,
    required this.controller,
    required this.builder,
  });

  final AppController controller;
  final WidgetBuilder builder;

  @override
  State<SettingsDataGate> createState() => _SettingsDataGateState();
}

class _SettingsDataGateState extends State<SettingsDataGate> {
  bool _loading = true;
  bool _available = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _load();
    });
  }

  Future<void> _load() async {
    if (mounted) setState(() => _loading = true);
    final available = await widget.controller.loadSettingsData(
      allowDemoFallback: false,
    );
    if (!mounted) return;
    setState(() {
      _loading = false;
      _available = available;
    });
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Scaffold(
        key: ValueKey('settings-loading'),
        backgroundColor: AppColors.background,
        body: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              CircularProgressIndicator(),
              SizedBox(height: AppSpacing.lg),
              Text('Loading settings...'),
            ],
          ),
        ),
      );
    }
    if (!_available) {
      return Scaffold(
        key: const ValueKey('settings-unavailable'),
        backgroundColor: AppColors.background,
        appBar: AppBar(
          title: const Text('Settings'),
          backgroundColor: AppColors.background,
          surfaceTintColor: Colors.transparent,
        ),
        body: Center(
          child: Padding(
            padding: const EdgeInsets.all(AppSpacing.xl),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.cloud_off_outlined, size: 48),
                const SizedBox(height: AppSpacing.lg),
                const Text(
                  'Settings data is temporarily unavailable.',
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: AppSpacing.sm),
                Text(
                  widget.controller.settingsError ??
                      'The backend did not return the required settings.',
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: AppSpacing.xl),
                FilledButton.icon(
                  onPressed: _load,
                  icon: const Icon(Icons.refresh),
                  label: const Text('Retry settings'),
                ),
              ],
            ),
          ),
        ),
      );
    }
    return widget.builder(context);
  }
}
