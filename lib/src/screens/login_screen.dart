import 'package:flutter/material.dart';

import '../app_controller.dart';
import '../theme.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key, required this.controller});

  final AppController controller;

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final _secretController = TextEditingController();

  @override
  void dispose() {
    _secretController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final secret = _secretController.text.trim();
    if (secret.isEmpty || widget.controller.authenticating) return;
    await widget.controller.login(secret);
    if (mounted && widget.controller.authenticated) {
      _secretController.clear();
    }
  }

  @override
  Widget build(BuildContext context) => Scaffold(
    backgroundColor: AppColors.background,
    body: SafeArea(
      child: Center(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(AppSpacing.xl),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: Card(
              child: Padding(
                padding: const EdgeInsets.all(AppSpacing.xl),
                child: AnimatedBuilder(
                  animation: widget.controller,
                  builder: (context, _) => Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      const Icon(
                        Icons.lock_outline,
                        size: 48,
                        color: AppColors.accent,
                      ),
                      const SizedBox(height: AppSpacing.lg),
                      Text(
                        'Sign in to DevEnglish',
                        textAlign: TextAlign.center,
                        style: Theme.of(context).textTheme.headlineSmall,
                      ),
                      const SizedBox(height: AppSpacing.sm),
                      Text(
                        'Your session is protected by a secure browser cookie.',
                        textAlign: TextAlign.center,
                        style: Theme.of(context).textTheme.bodyMedium,
                      ),
                      if (widget.controller.authError != null) ...[
                        const SizedBox(height: AppSpacing.lg),
                        Text(
                          widget.controller.authError!,
                          textAlign: TextAlign.center,
                          style: Theme.of(context).textTheme.bodyMedium
                              ?.copyWith(color: AppColors.error),
                        ),
                      ],
                      const SizedBox(height: AppSpacing.lg),
                      TextField(
                        controller: _secretController,
                        obscureText: true,
                        autofocus: true,
                        enabled: !widget.controller.authenticating,
                        autofillHints: const [AutofillHints.password],
                        decoration: const InputDecoration(
                          labelText: 'Access secret',
                          prefixIcon: Icon(Icons.key_outlined),
                        ),
                        onSubmitted: (_) => _submit(),
                      ),
                      const SizedBox(height: AppSpacing.lg),
                      FilledButton.icon(
                        onPressed: widget.controller.authenticating
                            ? null
                            : _submit,
                        icon: widget.controller.authenticating
                            ? const SizedBox(
                                width: 18,
                                height: 18,
                                child: CircularProgressIndicator(
                                  strokeWidth: 2,
                                ),
                              )
                            : const Icon(Icons.login),
                        label: Text(
                          widget.controller.authenticating
                              ? 'Signing in…'
                              : 'Sign in',
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    ),
  );
}
