import 'package:flutter/foundation.dart';

import 'workspace_demo.dart';
import 'workspace_models.dart';

abstract interface class AssistantGateway {
  Future<AssistantResponse> ask(String prompt, WorkspaceData data);
}

class DemoAssistantGateway implements AssistantGateway {
  const DemoAssistantGateway();

  @override
  Future<AssistantResponse> ask(String prompt, WorkspaceData data) async {
    await Future<void>.delayed(const Duration(milliseconds: 120));
    final lower = prompt.toLowerCase();

    if (lower.contains('unknown') || lower.contains('không có')) {
      return _unknownResponse();
    }

    if (lower.contains('task') ||
        lower.contains('việc') ||
        lower.contains('tiến độ')) {
      final citation = _citationFor(data, 'evidence-wave3-task');
      if (citation == null) return _unknownResponse();
      return AssistantResponse(
        answer:
            'The current high-priority task is to finish the Today, Work and Knowledge workspace UI. The next safe step is to validate the text assistant flow and its citations before adding voice features.',
        evidence: [citation],
        unknowns: const [],
        staleSources: const [],
        suggestedActions: const [
          'Review the UI flow',
          'Check citation visibility',
        ],
        actionReceipts: const [],
      );
    }

    if (lower.contains('today') || lower.contains('workspace')) {
      final citation = _citationFor(data, 'evidence-wave3-ui');
      if (citation == null) return _unknownResponse();
      return AssistantResponse(
        answer:
            'Today is the default workspace, with Work, Knowledge and Learning available as separate destinations.',
        evidence: [citation],
        unknowns: const [],
        staleSources: const [],
        suggestedActions: const [],
        actionReceipts: const [],
      );
    }

    if (lower.contains('mcp') || lower.contains('rest')) {
      final citation = _citationFor(data, 'evidence-wave3-mcp');
      if (citation == null) return _unknownResponse();
      return AssistantResponse(
        answer:
            'MCP read tools reuse the same application-service semantics as REST, while write tools stay behind explicit confirmation.',
        evidence: [citation],
        unknowns: const [],
        staleSources: const [],
        suggestedActions: const [],
        actionReceipts: const [],
      );
    }

    return _unknownResponse();
  }

  AssistantResponse _unknownResponse() => const AssistantResponse(
    answer:
        'I cannot verify that from the connected project sources yet. I will keep it as unknown instead of inventing a project fact.',
    evidence: [],
    unknowns: ['The requested detail is not present in the preview source.'],
    staleSources: [],
    suggestedActions: [
      'Add a source or record a decision before relying on this detail.',
    ],
    actionReceipts: [],
  );

  AssistantCitation? _citationFor(WorkspaceData data, String evidenceId) {
    for (final source in data.sources) {
      for (final evidence in source.evidence) {
        if (evidence.id == evidenceId) {
          return AssistantCitation(
            sourceId: source.id,
            sourceTitle: source.title,
            location: evidence.location,
            excerpt: evidence.excerpt,
            origin: data.origin,
          );
        }
      }
    }
    return null;
  }
}

class WorkspaceController extends ChangeNotifier {
  WorkspaceController({WorkspaceData? data, AssistantGateway? assistantGateway})
    : data = data ?? WorkspaceDemo.data,
      _assistantGateway = assistantGateway ?? const DemoAssistantGateway(),
      _conversation = List<AssistantTurn>.of(
        (data ?? WorkspaceDemo.data).initialConversation,
      );

  final WorkspaceData data;
  final AssistantGateway _assistantGateway;
  final List<AssistantTurn> _conversation;
  bool _sending = false;
  String? _error;

  List<AssistantTurn> get conversation => List.unmodifiable(_conversation);
  bool get sending => _sending;
  String? get error => _error;

  Future<void> ask(String prompt) async {
    final trimmed = prompt.trim();
    if (trimmed.isEmpty || _sending) return;

    _conversation.add(
      AssistantTurn(
        id: 'user-${_conversation.length}',
        role: 'user',
        content: trimmed,
      ),
    );
    _sending = true;
    _error = null;
    notifyListeners();

    try {
      final response = await _assistantGateway.ask(trimmed, data);
      _conversation.add(
        AssistantTurn(
          id: 'assistant-${_conversation.length}',
          role: 'assistant',
          content: response.answer,
          citations: response.evidence,
          unknowns: response.unknowns,
          suggestedActions: response.suggestedActions,
        ),
      );
    } catch (_) {
      _error =
          'The assistant is unavailable. Your message is still visible in this session.';
    } finally {
      _sending = false;
      notifyListeners();
    }
  }
}
