'use client'

import { useState } from 'react'
import { differenceInCalendarDays, endOfDay, format, isValid, parse, startOfDay, subDays, subHours, subMonths } from 'date-fns'
import type { DateRange as DayPickerRange } from 'react-day-picker'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

export interface DateRange {
  from: Date
  to: Date
}

const INPUT_FORMAT = 'MM/dd/yyyy'

export const DATE_PRESETS: { label: string; range: (now: Date) => DateRange }[] = [
  { label: 'Last hour', range: (now) => ({ from: subHours(now, 1), to: now }) },
  { label: 'Today', range: (now) => ({ from: startOfDay(now), to: endOfDay(now) }) },
  ...[7, 30, 60, 90].map((days) => ({
    label: `Last ${days} days`,
    range: (now: Date) => ({ from: startOfDay(subDays(now, days - 1)), to: endOfDay(now) }),
  })),
]

function toText(d: Date | undefined): string {
  return d ? format(d, INPUT_FORMAT) : ''
}

function parseInput(text: string): Date | null {
  if (!/^\d{2}\/\d{2}\/\d{4}$/.test(text.trim())) return null
  const d = parse(text.trim(), INPUT_FORMAT, new Date())
  return isValid(d) ? d : null
}

export interface DateRangeDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title?: string
  value: DateRange | null
  onApply: (range: DateRange) => void
  onClear?: () => void
  /** Longest selectable range in days (inclusive); Apply is disabled beyond it. */
  maxDays?: number
}

/** Created-date picker: start/end inputs, two-month range calendar, preset chips (spec §5.2). */
export function DateRangeDialog({
  open,
  onOpenChange,
  title = 'Filter by created date',
  value,
  onApply,
  onClear,
  maxDays,
}: DateRangeDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[calc(100%-2rem)] sm:max-w-[640px]">
        <DialogHeader>
          <DialogTitle className="text-xl font-semibold">{title}</DialogTitle>
          <DialogDescription className="sr-only">Choose a start and end date</DialogDescription>
        </DialogHeader>
        {/* Mounted only while open, so the draft resets from `value` each time the dialog opens. */}
        <DateRangeBody
          value={value}
          maxDays={maxDays}
          onCancel={() => onOpenChange(false)}
          onApply={(r) => {
            onApply(r)
            onOpenChange(false)
          }}
          onClear={
            onClear && value
              ? () => {
                  onClear()
                  onOpenChange(false)
                }
              : undefined
          }
        />
      </DialogContent>
    </Dialog>
  )
}

function DateRangeBody({
  value,
  maxDays,
  onCancel,
  onApply,
  onClear,
}: {
  value: DateRange | null
  maxDays?: number
  onCancel: () => void
  onApply: (range: DateRange) => void
  onClear?: () => void
}) {
  const [from, setFrom] = useState<Date | undefined>(value?.from)
  const [to, setTo] = useState<Date | undefined>(value?.to)
  const [fromText, setFromText] = useState(toText(value?.from))
  const [toTextValue, setToTextValue] = useState(toText(value?.to))
  const [preset, setPreset] = useState<string | null>(null)
  const [month, setMonth] = useState<Date>(subMonths(value?.to ?? new Date(), 1))

  function setRange(f: Date | undefined, t: Date | undefined, presetLabel: string | null = null) {
    setFrom(f)
    setTo(t)
    setFromText(toText(f))
    setToTextValue(toText(t))
    setPreset(presetLabel)
  }

  function onCalendarSelect(range: DayPickerRange | undefined) {
    setRange(range?.from && startOfDay(range.from), range?.to && endOfDay(range.to))
  }

  function onFromInput(text: string) {
    setFromText(text)
    setPreset(null)
    const d = parseInput(text)
    setFrom(d ? startOfDay(d) : undefined)
    if (d) setMonth(d)
  }

  function onToInput(text: string) {
    setToTextValue(text)
    setPreset(null)
    const d = parseInput(text)
    setTo(d ? endOfDay(d) : undefined)
  }

  const ordered = !!from && !!to && from.getTime() <= to.getTime()
  const tooLong = ordered && maxDays != null && differenceInCalendarDays(to!, from!) + 1 > maxDays
  const valid = ordered && !tooLong

  return (
    <>
      <div className="grid grid-cols-2 gap-4">
        <div className="grid gap-1.5">
          <Label htmlFor="date-range-start">Start date</Label>
          <Input
            id="date-range-start"
            placeholder="MM/DD/YYYY"
            inputMode="numeric"
            value={fromText}
            onChange={(e) => onFromInput(e.target.value)}
          />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="date-range-end">End date</Label>
          <Input
            id="date-range-end"
            placeholder="MM/DD/YYYY"
            inputMode="numeric"
            value={toTextValue}
            onChange={(e) => onToInput(e.target.value)}
          />
        </div>
      </div>
      <div className="flex justify-center">
        <Calendar
          mode="range"
          numberOfMonths={2}
          month={month}
          onMonthChange={setMonth}
          selected={{ from, to }}
          onSelect={onCalendarSelect}
          disabled={{ after: new Date() }}
        />
      </div>
      <div className="flex flex-wrap gap-2">
        {DATE_PRESETS.map((p) => (
          <button
            key={p.label}
            type="button"
            aria-pressed={preset === p.label}
            onClick={() => {
              const r = p.range(new Date())
              setRange(r.from, r.to, p.label)
              setMonth(subMonths(r.to, 1))
            }}
            className={cn(
              'h-8 rounded-full border border-border px-3 text-sm font-medium text-foreground hover:bg-muted',
              preset === p.label && 'border-primary bg-accent text-primary',
            )}
          >
            {p.label}
          </button>
        ))}
      </div>
      {tooLong && <p className="text-sm text-destructive">Select up to {maxDays} days</p>}
      <DialogFooter className="items-center sm:justify-between">
        <div>
          {onClear && (
            <Button type="button" variant="link" className="px-0" onClick={onClear}>
              Clear
            </Button>
          )}
        </div>
        <div className="flex gap-2">
          <Button type="button" variant="secondary" onClick={onCancel}>
            Cancel
          </Button>
          <Button type="button" disabled={!valid} onClick={() => valid && onApply({ from: from!, to: to! })}>
            Apply
          </Button>
        </div>
      </DialogFooter>
    </>
  )
}
