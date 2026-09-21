import React from 'react'

export interface MshLogoProps extends React.SVGProps<SVGSVGElement> {
  size?: number
  className?: string
  variant?: 'badge' | 'glyph' | 'monochrome'
}

export function MshLogo({ size = 32, className = '', variant = 'badge', ...props }: MshLogoProps) {
  if (variant === 'glyph') {
    return (
      <svg
        width={size}
        height={size}
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        className={className}
        {...props}
      >
        <polyline points="4 6 12 12 4 18" />
        <line x1="14" y1="18" x2="20" y2="18" />
      </svg>
    )
  }

  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 48 48"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      {...props}
    >
      <rect
        width="48"
        height="48"
        rx="11"
        fill="#5c3a2e"
      />
      {/* Shell chevron prompt */}
      <path
        d="M14 17L22 25L14 33"
        stroke="#ffffff"
        strokeWidth="3.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      {/* Shell cursor underscore */}
      <line
        x1="26"
        y1="33"
        x2="35"
        y2="33"
        stroke="#ffffff"
        strokeWidth="3.5"
        strokeLinecap="round"
      />
    </svg>
  )
}
