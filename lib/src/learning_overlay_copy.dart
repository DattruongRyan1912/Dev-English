/// Deterministic, level-aware copy for the optional English learning layer.
///
/// This is presentation guidance only. It must never be written into
/// canonical work entities or treated as evidence by the assistant.
class LearningOverlayCopy {
  const LearningOverlayCopy({
    required this.level,
    required this.label,
    required this.explanation,
    required this.supportStarter,
    required this.workflowStarter,
    required this.taskStarter,
    required this.followUp,
  });

  final String level;
  final String label;
  final String explanation;
  final String supportStarter;
  final String workflowStarter;
  final String taskStarter;
  final String followUp;

  static LearningOverlayCopy forLevel(String value) {
    return switch (value.trim().toUpperCase()) {
      'A1' => const LearningOverlayCopy(
        level: 'A1',
        label: 'guided',
        explanation:
            'Nói một ý ngắn về việc đang xảy ra. Dùng câu mẫu và thay từng chỗ trống.',
        supportStarter: 'The problem is ____. I will check ____.',
        workflowStarter: 'I will ____ because ____.',
        taskStarter: 'I need to ____. I will check ____.',
        followUp: 'What will you check first?',
      ),
      'A2' => const LearningOverlayCopy(
        level: 'A2',
        label: 'guided',
        explanation:
            'Dùng hai câu ngắn: câu đầu nói vấn đề, câu sau nói việc bạn sẽ làm.',
        supportStarter: 'The issue happens when ____. I will check ____ next.',
        workflowStarter: 'The next step is ____. I will check ____.',
        taskStarter: 'I need to ____ because ____.',
        followUp: 'What is one detail you can verify?',
      ),
      'B2' => const LearningOverlayCopy(
        level: 'B2',
        label: 'structured',
        explanation:
            'Nêu nguyên nhân có khả năng xảy ra, tác động và cách kiểm tra trước khi sửa.',
        supportStarter:
            'The issue appears when ____. It affects ____. I will verify ____ before changing ____.',
        workflowStarter:
            'The issue appears to be ____. I will verify ____ before ____.',
        taskStarter:
            'I will address ____ because ____. Before changing it, I will verify ____.',
        followUp: 'Which evidence would confirm that next step?',
      ),
      'C1' => const LearningOverlayCopy(
        level: 'C1',
        label: 'precise',
        explanation:
            'Giải thích ngắn gọn nhưng chính xác: phân biệt quan sát, giả thuyết và bước xác minh.',
        supportStarter:
            'The observed failure is ____. The likely cause is ____, which I will validate by checking ____.',
        workflowStarter:
            'The evidence indicates ____, but I will validate the hypothesis by checking ____.',
        taskStarter:
            'I will address ____ after validating ____ because the main risk is ____.',
        followUp:
            'What evidence would distinguish the hypothesis from the symptom?',
      ),
      'C2' => const LearningOverlayCopy(
        level: 'C2',
        label: 'advanced',
        explanation:
            'Giữ lập luận ngắn nhưng có sắc thái: nêu giới hạn của bằng chứng và quyết định tiếp theo.',
        supportStarter:
            'The current evidence suggests ____, although ____ remains unverified; I will resolve that by ____.',
        workflowStarter:
            'The evidence currently supports ____, while ____ remains uncertain; I will resolve it by ____.',
        taskStarter:
            'I will proceed with ____ provided that ____ is verified, because ____.',
        followUp: 'What uncertainty still needs evidence before you commit?',
      ),
      _ => const LearningOverlayCopy(
        level: 'B1',
        label: 'practical',
        explanation:
            'Khi gặp một nhiệm vụ kỹ thuật, bạn chỉ cần nói ba ý: chuyện gì xảy ra, ảnh hưởng là gì và bước an toàn tiếp theo là gì.',
        supportStarter:
            'The issue happens when ____. It affects ____. My next step is to ____.',
        workflowStarter: 'I will ____ because ____. My next step is to ____.',
        taskStarter: 'I need to ____ because ____. The next step is to ____.',
        followUp:
            'Hãy điền một chi tiết bạn có thể kiểm tra được vào mỗi chỗ trống.',
      ),
    };
  }
}
