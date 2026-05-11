import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  listSubjects,
  createSubject,
  updateSubject,
  deleteSubject,
} from '../api/subjects'
import type {
  SubjectResult,
  CreateSubjectPayload,
  UpdateSubjectPayload,
} from '../api/subjects'

/**
 * Subjects store.
 * Manages the subject list for the current project and CRUD operations.
 */
export const useSubjectsStore = defineStore('subjects', () => {
  const subjects = ref<SubjectResult[]>([])
  const loading = ref(false)
  const currentProjectId = ref<string | null>(null)

  async function load(projectId: string): Promise<void> {
    currentProjectId.value = projectId
    loading.value = true
    try {
      const result = await listSubjects(projectId)
      subjects.value = result.subjects
    } finally {
      loading.value = false
    }
  }

  async function create(
    projectId: string,
    payload: CreateSubjectPayload,
  ): Promise<SubjectResult> {
    const subject = await createSubject(projectId, payload)
    await load(projectId)
    return subject
  }

  async function update(
    projectId: string,
    id: string,
    payload: UpdateSubjectPayload,
  ): Promise<SubjectResult> {
    const subject = await updateSubject(projectId, id, payload)
    await load(projectId)
    return subject
  }

  async function remove(
    projectId: string,
    id: string,
    hardDelete = false,
  ): Promise<void> {
    await deleteSubject(projectId, id, hardDelete)
    await load(projectId)
  }

  return {
    subjects,
    loading,
    currentProjectId,
    load,
    create,
    update,
    remove,
  }
})
