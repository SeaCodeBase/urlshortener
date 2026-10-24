'use client'

import { useId, useRef, useState } from 'react'
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import { SortableContext, arrayMove, rectSortingStrategy, sortableKeyboardCoordinates, useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import type { DashboardData, DashboardLayout, DashboardModule, ModuleKey, StatsUnit } from '@/types'
import { ModuleCard } from './ModuleCard'

/** Modules that span both grid columns. */
const WIDE: ModuleKey[] = ['over_time']

interface Shared {
  data: DashboardData
  compare: boolean
  unit: StatsUnit
}

function SortableModule({ module, onChangeViz, ...shared }: Shared & { module: DashboardModule; onChangeViz: (m: DashboardModule) => void }) {
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({ id: module.key })
  return (
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Translate.toString(transform), transition }}
      className={cn(WIDE.includes(module.key) && 'lg:col-span-2', isDragging && 'relative z-10 opacity-80 shadow-lg')}
    >
      <ModuleCard
        moduleKey={module.key}
        viz={module.viz}
        {...shared}
        onChangeViz={(viz) => onChangeViz({ key: module.key, viz })}
        dragHandleProps={{ ref: setActivatorNodeRef, ...attributes, ...listeners }}
      />
    </div>
  )
}

/**
 * Two-column grid of the dashboard's modules in `layout` order. Dragging a handle (pointer or keyboard)
 * reorders the modules; reorders and visualization changes show at once and persist through
 * `onLayoutChange`, reverting with a toast when it rejects.
 */
export function ModuleGrid({
  layout,
  onLayoutChange,
  ...shared
}: Shared & { layout: DashboardLayout; onLayoutChange: (layout: DashboardLayout) => Promise<unknown> }) {
  const dndId = useId()
  const [pending, setPending] = useState<DashboardLayout | null>(null)
  const latest = useRef(0)
  const shown = pending ?? layout

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  function persist(next: DashboardLayout) {
    const call = ++latest.current
    setPending(next)
    onLayoutChange(next).then(
      () => call === latest.current && setPending(null),
      () => {
        if (call === latest.current) setPending(null)
        toast.error("Couldn't save the dashboard layout")
      },
    )
  }

  function onDragEnd({ active, over }: DragEndEvent) {
    if (!over || active.id === over.id) return
    const keys = shown.modules.map((m) => m.key)
    const from = keys.indexOf(active.id as ModuleKey)
    const to = keys.indexOf(over.id as ModuleKey)
    if (from < 0 || to < 0) return
    persist({ modules: arrayMove(shown.modules, from, to) })
  }

  function changeViz(changed: DashboardModule) {
    persist({ modules: shown.modules.map((m) => (m.key === changed.key ? changed : m)) })
  }

  return (
    <DndContext id={dndId} sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
      <SortableContext items={shown.modules.map((m) => m.key)} strategy={rectSortingStrategy}>
        <div className="grid gap-4 lg:grid-cols-2">
          {shown.modules.map((m) => (
            <SortableModule key={m.key} module={m} onChangeViz={changeViz} {...shared} />
          ))}
        </div>
      </SortableContext>
    </DndContext>
  )
}
