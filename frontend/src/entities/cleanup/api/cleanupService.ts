import GmHttpService from '@/shared/api/gmHttpService'
import type { CleanupJob, CleanupRun, CreateJobRequest, PreviewResponse, UpdateJobRequest } from '../model/types'

class CleanupService extends GmHttpService {
  preview(filter: unknown) {
    return this.post<PreviewResponse>('/api/v1/cleanup/preview', { filter })
  }

  listJobs() {
    return this.get<CleanupJob[]>('/api/v1/cleanup/jobs')
  }

  createJob(data: CreateJobRequest) {
    return this.post<{ id: string }>('/api/v1/cleanup/jobs', data)
  }

  updateJob(id: string, data: UpdateJobRequest) {
    return this.put<{ message: string }>(`/api/v1/cleanup/jobs/${id}`, data)
  }

  deleteJob(id: string) {
    return this.delete<{ message: string }>(`/api/v1/cleanup/jobs/${id}`)
  }

  runJob(id: string) {
    return this.post<{ run_id: string; status: string }>(`/api/v1/cleanup/jobs/${id}/run`)
  }

  listRuns(jobId: string) {
    return this.get<CleanupRun[]>(`/api/v1/cleanup/jobs/${jobId}/runs`)
  }
}

export const cleanupService = new CleanupService()
