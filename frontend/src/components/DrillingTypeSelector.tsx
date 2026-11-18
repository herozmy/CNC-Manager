import { useNavigate } from 'react-router-dom'

export default function DrillingTypeSelector({ value }: { value: 'polar-drilling' | 'line-drilling' | 'grid-drilling' }) {
  const navigate = useNavigate()
  return <div className="panel space-y-1.5 p-4 md:p-5"><label htmlFor="drilling-type" className="block text-sm font-medium text-body">钻孔类型</label><select id="drilling-type" className="h-12 w-full rounded-md border border-line-strong bg-surface px-3 text-base text-ink focus:outline-2 focus:outline-accent" value={value} onChange={e => navigate(`/tools/gcode/${e.target.value}`)}><option value="polar-drilling">极坐标圆打孔</option><option value="line-drilling">直线等距钻孔</option><option value="grid-drilling">矩形阵列钻孔</option></select></div>
}
