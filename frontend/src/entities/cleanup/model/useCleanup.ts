'use client'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { cleanupService } from '../api/cleanupService'
import type { CreateJobRequest, UpdateJobRequest } from './types'

export const CLEANUP_JOBS_KEY = ['cleanupJobs'] as const
export const CLEANUP_RUNS_KEY = (jobId: string) => ['cleanupRuns', jobId] as const

export function useCleanupJobs() {
  return useQuery({
    queryKey: CLEANUP_JOBS_KEY,
    queryFn: () => cleanupService.listJobs(),
  })
}

export function useCleanupRuns(jobId: string) {
  return useQuery({
    queryKey: CLEANUP_RUNS_KEY(jobId),
    queryFn: () => cleanupService.listRuns(jobId),
    enabled: !!jobId,
    refetchInterval: 5000,
  })
}

export function useCreateCleanupJob() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: CreateJobRequest) => cleanupService.createJob(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: CLEANUP_JOBS_KEY }),
  })
}

export function useUpdateCleanupJob() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateJobRequest }) =>
      cleanupService.updateJob(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: CLEANUP_JOBS_KEY }),
  })
}

export function useDeleteCleanupJob() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => cleanupService.deleteJob(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: CLEANUP_JOBS_KEY }),
  })
}

export function useRunCleanupJob() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => cleanupService.runJob(id),
    onSuccess: (_, jobId) => qc.invalidateQueries({ queryKey: CLEANUP_RUNS_KEY(jobId) }),
  })
}
