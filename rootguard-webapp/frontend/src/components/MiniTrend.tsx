// =====================================================
// File: frontend/src/components/MiniTrend.tsx
// Purpose: Smooth gradient SVG trend chart
// =====================================================

interface Props {
  data: number[];
}

export default function MiniTrend({ data }: Props) {

  const max = Math.max(...data);
  const width = 120;
  const height = 40;

  // Convert values to coordinates
  const points = data.map((value, index) => {
    const x = (index / (data.length - 1)) * width;
    const y = height - (value / max) * height;
    return { x, y };
  });

  // Build smooth curve path using quadratic Bezier
  let path = `M ${points[0].x} ${points[0].y}`;

  for (let i = 1; i < points.length; i++) {
    const prev = points[i - 1];
    const current = points[i];

    const cx = (prev.x + current.x) / 2;
    const cy = (prev.y + current.y) / 2;

    path += ` Q ${prev.x} ${prev.y}, ${cx} ${cy}`;
  }

  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`}>

      <defs>
        <linearGradient id="trendGradient" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#22c55e" stopOpacity="0.6" />
          <stop offset="100%" stopColor="#22c55e" stopOpacity="0" />
        </linearGradient>
      </defs>

      {/* Fill Area */}
      <path
        d={`${path} L ${width} ${height} L 0 ${height} Z`}
        fill="url(#trendGradient)"
      />

      {/* Line */}
      <path
        d={path}
        fill="none"
        stroke="#22c55e"
        strokeWidth="2.5"
        strokeLinecap="round"
      />

    </svg>
  );
}