typedef JsonObject = Map<String, dynamic>;

const String v1ContractVersion = 'v1';

String _version(Object? value) =>
    value is String && value.isNotEmpty ? value : v1ContractVersion;

String _string(Object? value) => value is String ? value : '';

String? _nullableString(Object? value) => value is String ? value : null;

double? _nullableDouble(Object? value) =>
    value is num ? value.toDouble() : null;

DateTime? _dateTime(Object? value) =>
    value is String ? DateTime.tryParse(value) : null;

String? _dateTimeJson(DateTime? value) => value?.toUtc().toIso8601String();

JsonObject _object(Object? value) => value is Map
    ? <String, dynamic>{
        for (final entry in value.entries)
          if (entry.key is String) entry.key as String: entry.value,
      }
    : <String, dynamic>{};

List<T> _objects<T>(Object? value, T Function(JsonObject) decode) =>
    value is List
    ? <T>[
        for (final item in value)
          if (item is Map) decode(_object(item)),
      ]
    : <T>[];

class Today {
  Today({
    required this.date,
    this.version = v1ContractVersion,
    this.summary,
    this.focusTaskId,
    this.projects = const <Project>[],
    this.tasks = const <Task>[],
    this.decisions = const <Decision>[],
  });

  final String version;
  final String date;
  final String? summary;
  final String? focusTaskId;
  final List<Project> projects;
  final List<Task> tasks;
  final List<Decision> decisions;

  factory Today.fromJson(JsonObject json) => Today(
    version: _version(json['version']),
    date: _string(json['date']),
    summary: _nullableString(json['summary']),
    focusTaskId: _nullableString(json['focusTaskId']),
    projects: _objects(json['projects'], Project.fromJson),
    tasks: _objects(json['tasks'], Task.fromJson),
    decisions: _objects(json['decisions'], Decision.fromJson),
  );

  JsonObject toJson() => <String, dynamic>{
    'version': version,
    'date': date,
    'summary': summary,
    'focusTaskId': focusTaskId,
    'projects': projects.map((value) => value.toJson()).toList(),
    'tasks': tasks.map((value) => value.toJson()).toList(),
    'decisions': decisions.map((value) => value.toJson()).toList(),
  };
}

class Project {
  Project({
    required this.id,
    required this.name,
    this.version = v1ContractVersion,
    this.description,
    this.status,
    this.repositoryUrl,
    this.updatedAt,
  });

  final String version;
  final String id;
  final String name;
  final String? description;
  final String? status;
  final String? repositoryUrl;
  final DateTime? updatedAt;

  factory Project.fromJson(JsonObject json) => Project(
    version: _version(json['version']),
    id: _string(json['id']),
    name: _string(json['name']),
    description: _nullableString(json['description']),
    status: _nullableString(json['status']),
    repositoryUrl: _nullableString(json['repositoryUrl']),
    updatedAt: _dateTime(json['updatedAt']),
  );

  JsonObject toJson() => <String, dynamic>{
    'version': version,
    'id': id,
    'name': name,
    'description': description,
    'status': status,
    'repositoryUrl': repositoryUrl,
    'updatedAt': _dateTimeJson(updatedAt),
  };
}

class Task {
  Task({
    required this.id,
    required this.title,
    required this.status,
    this.version = v1ContractVersion,
    this.projectId,
    this.description,
    this.priority,
    this.dueDate,
    this.updatedAt,
  });

  final String version;
  final String id;
  final String? projectId;
  final String title;
  final String? description;
  final String status;
  final String? priority;
  final String? dueDate;
  final DateTime? updatedAt;

  factory Task.fromJson(JsonObject json) => Task(
    version: _version(json['version']),
    id: _string(json['id']),
    projectId: _nullableString(json['projectId']),
    title: _string(json['title']),
    description: _nullableString(json['description']),
    status: _string(json['status']),
    priority: _nullableString(json['priority']),
    dueDate: _nullableString(json['dueDate']),
    updatedAt: _dateTime(json['updatedAt']),
  );

  JsonObject toJson() => <String, dynamic>{
    'version': version,
    'id': id,
    'projectId': projectId,
    'title': title,
    'description': description,
    'status': status,
    'priority': priority,
    'dueDate': dueDate,
    'updatedAt': _dateTimeJson(updatedAt),
  };
}

class Decision {
  Decision({
    required this.id,
    required this.title,
    required this.decision,
    this.version = v1ContractVersion,
    this.projectId,
    this.context,
    this.rationale,
    this.status,
    this.decidedAt,
  });

  final String version;
  final String id;
  final String? projectId;
  final String title;
  final String decision;
  final String? context;
  final String? rationale;
  final String? status;
  final DateTime? decidedAt;

  factory Decision.fromJson(JsonObject json) => Decision(
    version: _version(json['version']),
    id: _string(json['id']),
    projectId: _nullableString(json['projectId']),
    title: _string(json['title']),
    decision: _string(json['decision']),
    context: _nullableString(json['context']),
    rationale: _nullableString(json['rationale']),
    status: _nullableString(json['status']),
    decidedAt: _dateTime(json['decidedAt']),
  );

  JsonObject toJson() => <String, dynamic>{
    'version': version,
    'id': id,
    'projectId': projectId,
    'title': title,
    'decision': decision,
    'context': context,
    'rationale': rationale,
    'status': status,
    'decidedAt': _dateTimeJson(decidedAt),
  };
}

class EvidenceRef {
  EvidenceRef({
    required this.id,
    required this.sourceUri,
    this.version = v1ContractVersion,
    this.locator,
    this.quote,
  });

  final String version;
  final String id;
  final String sourceUri;
  final String? locator;
  final String? quote;

  factory EvidenceRef.fromJson(JsonObject json) => EvidenceRef(
    version: _version(json['version']),
    id: _string(json['id']),
    sourceUri: _string(json['sourceUri']),
    locator: _nullableString(json['locator']),
    quote: _nullableString(json['quote']),
  );

  JsonObject toJson() => <String, dynamic>{
    'version': version,
    'id': id,
    'sourceUri': sourceUri,
    'locator': locator,
    'quote': quote,
  };
}

class KnowledgeSearchResult {
  KnowledgeSearchResult({
    required this.id,
    required this.title,
    required this.snippet,
    this.version = v1ContractVersion,
    this.sourceUri,
    this.score,
    this.evidence = const <EvidenceRef>[],
  });

  final String version;
  final String id;
  final String title;
  final String snippet;
  final String? sourceUri;
  final double? score;
  final List<EvidenceRef> evidence;

  factory KnowledgeSearchResult.fromJson(JsonObject json) =>
      KnowledgeSearchResult(
        version: _version(json['version']),
        id: _string(json['id']),
        title: _string(json['title']),
        snippet: _string(json['snippet']),
        sourceUri: _nullableString(json['sourceUri']),
        score: _nullableDouble(json['score']),
        evidence: _objects(json['evidence'], EvidenceRef.fromJson),
      );

  JsonObject toJson() => <String, dynamic>{
    'version': version,
    'id': id,
    'title': title,
    'snippet': snippet,
    'sourceUri': sourceUri,
    'score': score,
    'evidence': evidence.map((value) => value.toJson()).toList(),
  };
}

class GroundedAnswer {
  GroundedAnswer({
    required this.answer,
    this.version = v1ContractVersion,
    this.confidence,
    this.evidence = const <EvidenceRef>[],
    this.searchResults = const <KnowledgeSearchResult>[],
  });

  final String version;
  final String answer;
  final double? confidence;
  final List<EvidenceRef> evidence;
  final List<KnowledgeSearchResult> searchResults;

  factory GroundedAnswer.fromJson(JsonObject json) => GroundedAnswer(
    version: _version(json['version']),
    answer: _string(json['answer']),
    confidence: _nullableDouble(json['confidence']),
    evidence: _objects(json['evidence'], EvidenceRef.fromJson),
    searchResults: _objects(
      json['searchResults'],
      KnowledgeSearchResult.fromJson,
    ),
  );

  JsonObject toJson() => <String, dynamic>{
    'version': version,
    'answer': answer,
    'confidence': confidence,
    'evidence': evidence.map((value) => value.toJson()).toList(),
    'searchResults': searchResults.map((value) => value.toJson()).toList(),
  };
}

class SuggestedAction {
  SuggestedAction({
    required this.kind,
    required this.title,
    required this.description,
    this.version = v1ContractVersion,
    this.needsConfirmation = false,
    Map<String, dynamic>? parameters,
  }) : parameters = parameters ?? <String, dynamic>{};

  final String version;
  final String kind;
  final String title;
  final String description;
  final bool needsConfirmation;
  final JsonObject parameters;

  factory SuggestedAction.fromJson(JsonObject json) => SuggestedAction(
    version: _version(json['version']),
    kind: _string(json['kind']),
    title: _string(json['title']),
    description: _string(json['description']),
    needsConfirmation: json['needsConfirmation'] == true,
    parameters: _object(json['parameters']),
  );

  JsonObject toJson() => <String, dynamic>{
    'version': version,
    'kind': kind,
    'title': title,
    'description': description,
    'needsConfirmation': needsConfirmation,
    'parameters': parameters,
  };
}

class ActionChallenge {
  ActionChallenge({
    required this.id,
    required this.action,
    required this.prompt,
    this.version = v1ContractVersion,
    this.targetType = '',
    this.targetId = '',
    this.actionHash = '',
    Map<String, dynamic>? parameters,
    this.expiresAt,
  }) : parameters = parameters ?? <String, dynamic>{};

  final String version;
  final String id;
  final String action;
  final String targetType;
  final String targetId;
  final String actionHash;
  final String prompt;
  final JsonObject parameters;
  final DateTime? expiresAt;

  factory ActionChallenge.fromJson(JsonObject json) => ActionChallenge(
    version: _version(json['version']),
    id: _string(json['id']),
    action: _string(json['action']),
    targetType: _string(json['targetType']),
    targetId: _string(json['targetId']),
    actionHash: _string(json['actionHash']),
    prompt: _string(json['prompt']),
    parameters: _object(json['parameters']),
    expiresAt: _dateTime(json['expiresAt']),
  );

  JsonObject toJson() => <String, dynamic>{
    'version': version,
    'id': id,
    'action': action,
    'targetType': targetType,
    'targetId': targetId,
    'actionHash': actionHash,
    'prompt': prompt,
    'parameters': parameters,
    'expiresAt': _dateTimeJson(expiresAt),
  };
}

class ActionReceipt {
  ActionReceipt({
    required this.id,
    required this.challengeId,
    required this.idempotencyKey,
    required this.action,
    required this.status,
    this.version = v1ContractVersion,
    this.targetType = '',
    this.targetId = '',
    this.message,
    Map<String, dynamic>? output,
    this.createdAt,
  }) : output = output ?? <String, dynamic>{};

  final String version;
  final String id;
  final String challengeId;
  final String idempotencyKey;
  final String action;
  final String targetType;
  final String targetId;
  final String status;
  final String? message;
  final JsonObject output;
  final DateTime? createdAt;

  factory ActionReceipt.fromJson(JsonObject json) => ActionReceipt(
    version: _version(json['version']),
    id: _string(json['id']),
    challengeId: _string(json['challengeId']),
    idempotencyKey: _string(json['idempotencyKey']),
    action: _string(json['action']),
    targetType: _string(json['targetType']),
    targetId: _string(json['targetId']),
    status: _string(json['status']),
    message: _nullableString(json['message']),
    output: _object(json['output']),
    createdAt: _dateTime(json['createdAt']),
  );

  JsonObject toJson() => <String, dynamic>{
    'version': version,
    'id': id,
    'challengeId': challengeId,
    'idempotencyKey': idempotencyKey,
    'action': action,
    'targetType': targetType,
    'targetId': targetId,
    'status': status,
    'message': message,
    'output': output,
    'createdAt': _dateTimeJson(createdAt),
  };
}

class AssistantResponse {
  AssistantResponse({
    required this.answer,
    this.version = v1ContractVersion,
    this.evidence = const <EvidenceRef>[],
    this.unknowns = const <String>[],
    this.staleSources = const <EvidenceRef>[],
    this.suggestedActions = const <SuggestedAction>[],
    this.actionReceipts = const <ActionReceipt>[],
  });

  final String version;
  final String answer;
  final List<EvidenceRef> evidence;
  final List<String> unknowns;
  final List<EvidenceRef> staleSources;
  final List<SuggestedAction> suggestedActions;
  final List<ActionReceipt> actionReceipts;

  factory AssistantResponse.fromJson(JsonObject json) => AssistantResponse(
    version: _version(json['version']),
    answer: _string(json['answer']),
    evidence: _objects(json['evidence'], EvidenceRef.fromJson),
    unknowns: json['unknowns'] is List
        ? (json['unknowns'] as List).whereType<String>().toList()
        : <String>[],
    staleSources: _objects(json['staleSources'], EvidenceRef.fromJson),
    suggestedActions: _objects(
      json['suggestedActions'],
      SuggestedAction.fromJson,
    ),
    actionReceipts: _objects(json['actionReceipts'], ActionReceipt.fromJson),
  );

  JsonObject toJson() => <String, dynamic>{
    'version': version,
    'answer': answer,
    'evidence': evidence.map((value) => value.toJson()).toList(),
    'unknowns': unknowns,
    'staleSources': staleSources.map((value) => value.toJson()).toList(),
    'suggestedActions': suggestedActions
        .map((value) => value.toJson())
        .toList(),
    'actionReceipts': actionReceipts.map((value) => value.toJson()).toList(),
  };
}
