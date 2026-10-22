export interface Task {
  id: number;
  title: string;
  description: string;
  status: "todo" | "doing" | "done";
  priority: number;
  tags: string[];
  dueDate: string;
  createdAt: string;
  updatedAt: string;
}

export interface TaskPage {
  items: Task[];
  page: number;
  limit: number;
}

export interface Stats {
  total: number;
  byStatus: Record<string, number>;
  overdue: number;
  avgPriority: number;
  topTags: { tag: string; count: number }[];
}

export interface WordCount {
  word: string;
  count: number;
}
