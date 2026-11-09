//! Declarative paint commands execute entirely within GPUI's frame.
use crate::*;
use gpui::{PathBuilder, canvas, point};
pub(crate) fn canvas_element(drawing: &pb::Drawing) -> gpui::AnyElement {
    let drawing = drawing.clone();
    canvas(
        |_, _, _| (),
        move |bounds, (), window, _| {
            window.with_content_mask(Some(gpui::ContentMask { bounds }), |window| {
                let point_at = |p: &pb::Point| bounds.origin + point(px(p.x), px(p.y));
                for command in drawing.commands {
                    match command.kind.unwrap() {
                        pb::draw_command::Kind::Rect(rect) => {
                            let b = rect.bounds.unwrap();
                            let dimensions = b.size.unwrap();
                            let bounds = Bounds::new(
                                point_at(&b.origin.unwrap()),
                                size(px(dimensions.width), px(dimensions.height)),
                            );
                            window.paint_quad(gpui::quad(
                                bounds,
                                px(rect.radius),
                                rect.fill
                                    .as_ref()
                                    .map(color)
                                    .unwrap_or_else(gpui::transparent_black),
                                px(rect.border_width),
                                rect.border
                                    .as_ref()
                                    .map(color)
                                    .unwrap_or_else(gpui::transparent_black),
                                gpui::BorderStyle::Solid,
                            ));
                        }
                        pb::draw_command::Kind::Path(path) => {
                            let mut builder = if path.stroke_width > 0.0 {
                                PathBuilder::stroke(px(path.stroke_width))
                            } else {
                                PathBuilder::fill()
                            };
                            for step in path.steps {
                                match pb::PathVerb::try_from(step.verb).unwrap() {
                                    pb::PathVerb::Move => {
                                        builder.move_to(point_at(step.point.as_ref().unwrap()))
                                    }
                                    pb::PathVerb::Line => {
                                        builder.line_to(point_at(step.point.as_ref().unwrap()))
                                    }
                                    pb::PathVerb::Quadratic => builder.curve_to(
                                        point_at(step.point.as_ref().unwrap()),
                                        point_at(step.control1.as_ref().unwrap()),
                                    ),
                                    pb::PathVerb::Cubic => builder.cubic_bezier_to(
                                        point_at(step.point.as_ref().unwrap()),
                                        point_at(step.control1.as_ref().unwrap()),
                                        point_at(step.control2.as_ref().unwrap()),
                                    ),
                                    pb::PathVerb::Close => builder.close(),
                                    _ => unreachable!(),
                                }
                            }
                            if let Ok(shape) = builder.build() {
                                window.paint_path(
                                    shape,
                                    path.color
                                        .as_ref()
                                        .map(color)
                                        .unwrap_or_else(gpui::transparent_black),
                                );
                            }
                        }
                    }
                }
            });
        },
    )
    .size_full()
    .into_any_element()
}
