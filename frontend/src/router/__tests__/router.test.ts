import { describe, it, expect } from 'vitest'
import router from '../index'

describe('Router', () => {
  const expectedRoutes = [
    { path: '/', name: 'projects' },
    { path: '/projects/:id', name: 'project-detail' },
    { path: '/projects/:id/subjects/:sid/samples', name: 'sample-grid' },
    { path: '/projects/:id/subjects/:sid/samples/:sampleId', name: 'sample-detail' },
    { path: '/jobs', name: 'jobs' },
    { path: '/studies', name: 'studies' },
    { path: '/export', name: 'export' },
    { path: '/queues', name: 'queues' },
    { path: '/accounts', name: 'accounts' },
    { path: '/settings', name: 'settings' },
  ]

  it('registers all PRD section 9 routes', () => {
    const registeredRoutes = router.getRoutes()
    for (const expected of expectedRoutes) {
      const found = registeredRoutes.find(r => r.name === expected.name)
      expect(found, `Route '${expected.name}' should exist`).toBeDefined()
      expect(found!.path).toBe(expected.path)
    }
  })

  it(`has exactly ${expectedRoutes.length} routes`, () => {
    expect(router.getRoutes()).toHaveLength(expectedRoutes.length)
  })

  it.each(expectedRoutes)('route $name has breadcrumb meta', (route) => {
    const found = router.getRoutes().find(r => r.name === route.name)
    expect(found?.meta.breadcrumb).toBeTruthy()
  })
})
