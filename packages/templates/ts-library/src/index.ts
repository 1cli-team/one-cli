export interface Range {
  start: number;
  end: number;
}

export function clamp(value: number, min: number, max: number): number {
  if (min > max) {
    throw new Error("min must be less than or equal to max");
  }

  if (value < min) {
    return min;
  }

  if (value > max) {
    return max;
  }

  return value;
}

export function inRange(value: number, range: Range): boolean {
  return value >= range.start && value <= range.end;
}
