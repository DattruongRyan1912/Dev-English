import 'package:flutter/material.dart';

import 'src/app_controller.dart';
import 'src/screens/focus_screen.dart';
import 'src/screens/home_screen.dart';
import 'src/screens/login_screen.dart';
import 'src/screens/practice_screen.dart';
import 'src/screens/progress_screen.dart';
import 'src/screens/review_screen.dart';
import 'src/screens/copilot_screen.dart';
import 'src/screens/diagnostic_screen.dart';
import 'src/screens/roleplay_screen.dart';
import 'src/screens/settings_screen.dart';
import 'src/screens/speaking_screen.dart';
import 'src/screens/vocabulary_screen.dart';
import 'src/screens/work_import_screen.dart';
import 'src/theme.dart';

void main() {
  runApp(const DevEnglishApp());
}

class DevEnglishApp extends StatefulWidget {
  const DevEnglishApp({super.key});

  @override
  State<DevEnglishApp> createState() => _DevEnglishAppState();
}

class _DevEnglishAppState extends State<DevEnglishApp> {
  late final AppController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AppController()..load();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => MaterialApp(
    title: 'DevEnglish',
    debugShowCheckedModeBanner: false,
    theme: buildAppTheme(),
    home: _AppShell(controller: _controller),
  );
}

class _AppShell extends StatefulWidget {
  const _AppShell({required this.controller});
  final AppController controller;

  @override
  State<_AppShell> createState() => _AppShellState();
}

class _AppShellState extends State<_AppShell> {
  int _index = 0;

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: widget.controller,
      builder: (context, _) {
        if (widget.controller.loading) {
          return const Scaffold(
            body: Center(child: CircularProgressIndicator()),
          );
        }
        if (widget.controller.requiresAuthentication &&
            !widget.controller.authenticated) {
          return LoginScreen(controller: widget.controller);
        }
        if (widget.controller.usingDemo &&
            !widget.controller.demoFallbackEnabled) {
          return Scaffold(
            body: Center(
              child: Padding(
                padding: const EdgeInsets.all(32),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(Icons.cloud_off_outlined, size: 48),
                    const SizedBox(height: 16),
                    const Text(
                      'Backend production chưa sẵn sàng',
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: 8),
                    Text(
                      widget.controller.error ??
                          'Không thể tải dữ liệu thật từ backend.',
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: 20),
                    FilledButton.icon(
                      onPressed: widget.controller.load,
                      icon: const Icon(Icons.refresh),
                      label: const Text('Thử lại'),
                    ),
                  ],
                ),
              ),
            ),
          );
        }
        final pages = [
          HomeScreen(
            controller: widget.controller,
            onStartMission: _startMission,
            onSettings: _openSettings,
            onDiagnostic: _openDiagnostic,
            onCopilot: _openCopilot,
            onVocabulary: _openVocabulary,
          ),
          PracticeScreen(
            controller: widget.controller,
            onStartMission: _startMission,
            onWorkImport: _openWorkImport,
            onSpeaking: _openSpeaking,
            onRoleplay: _openRoleplay,
            onCopilot: _openCopilot,
            onVocabulary: _openVocabulary,
          ),
          ReviewScreen(controller: widget.controller),
          ProgressScreen(controller: widget.controller),
        ];
        return Scaffold(
          body: IndexedStack(index: _index, children: pages),
          bottomNavigationBar: NavigationBar(
            selectedIndex: _index,
            onDestinationSelected: (value) => setState(() => _index = value),
            destinations: const [
              NavigationDestination(
                icon: Icon(Icons.home_outlined),
                selectedIcon: Icon(Icons.home),
                label: 'Home',
              ),
              NavigationDestination(
                icon: Icon(Icons.edit_outlined),
                selectedIcon: Icon(Icons.edit),
                label: 'Practice',
              ),
              NavigationDestination(
                icon: Icon(Icons.replay_outlined),
                selectedIcon: Icon(Icons.replay),
                label: 'Review',
              ),
              NavigationDestination(
                icon: Icon(Icons.insights_outlined),
                selectedIcon: Icon(Icons.insights),
                label: 'Progress',
              ),
            ],
          ),
        );
      },
    );
  }

  void _startMission() {
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        fullscreenDialog: true,
        builder: (_) => FocusScreen(controller: widget.controller),
      ),
    );
  }

  void _openWorkImport() {
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => WorkImportScreen(controller: widget.controller),
      ),
    );
  }

  void _openSpeaking() {
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => SpeakingScreen(controller: widget.controller),
      ),
    );
  }

  void _openRoleplay([String? scenarioType]) {
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => RoleplayScreen(
          controller: widget.controller,
          scenarioType: scenarioType,
        ),
      ),
    );
  }

  void _openCopilot() {
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => CopilotScreen(controller: widget.controller),
      ),
    );
  }

  void _openVocabulary() {
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => VocabularyScreen(controller: widget.controller),
      ),
    );
  }

  void _openDiagnostic() {
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => DiagnosticScreen(controller: widget.controller),
      ),
    );
  }

  void _openSettings() {
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => SettingsScreen(controller: widget.controller),
      ),
    );
  }
}
