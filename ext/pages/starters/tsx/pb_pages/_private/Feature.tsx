// A component: import it from a page or layout. _private files are never served as routes.
export type FeatureProps = { icon: string; title: string; text: string }

export default function Feature({ icon, title, text }: FeatureProps) {
  return (
    <div class="card">
      <div class="icon">{icon}</div>
      <h3>{title}</h3>
      <p>{text}</p>
    </div>
  )
}
