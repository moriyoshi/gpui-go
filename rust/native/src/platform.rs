//! Application menus and asynchronous platform path dialogs.
use crate::*;
#[derive(Clone, PartialEq, gpui::Action)]
#[action(no_json,namespace=gpui_go)]
pub(crate) struct MenuAction {
    pub target: pb::FocusTarget,
    pub action: pb::Action,
    pub propagate: bool,
}
pub(crate) fn init(cx: &mut App, hub: Arc<Hub>) {
    cx.on_action(move |action: &MenuAction, cx| {
        if let Err(error) = hub.input(Message::Action {
            entity: action.target.view.clone().unwrap(),
            element: action.target.element_id,
            action: action.action.clone(),
        }) {
            hub.finish(error);
            cx.quit();
        }
        if action.propagate {
            cx.propagate();
        }
    });
}
fn menu_item(item: &pb::MenuItem) -> gpui::MenuItem {
    if item.separator {
        return gpui::MenuItem::Separator;
    }
    if !item.children.is_empty() {
        return gpui::MenuItem::Submenu(gpui::Menu {
            name: item.label.clone().into(),
            items: item.children.iter().map(menu_item).collect(),
            disabled: item.disabled,
        });
    }
    gpui::MenuItem::Action {
        name: item.label.clone().into(),
        action: Box::new(MenuAction {
            target: item.target.clone().unwrap(),
            action: item.action.clone().unwrap(),
            propagate: false,
        }),
        os_action: None,
        checked: item.checked,
        disabled: item.disabled,
    }
}
pub(crate) fn handles(command: &pb::Command) -> bool {
    matches!(
        command.kind,
        Some(
            pb::command::Kind::SetMenus(_)
                | pb::command::Kind::PromptPaths(_)
                | pb::command::Kind::PromptSavePath(_)
        )
    )
}
pub(crate) fn command(
    command: &pb::Command,
    windows: &mut Windows,
    cx: &mut App,
    hub: Arc<Hub>,
) -> Result<pb::CommandResult, pb::ProtocolError> {
    use pb::command::Kind;
    if let Kind::SetMenus(menus) = command.kind.as_ref().unwrap() {
        cx.set_menus(menus.menus.iter().map(|m| gpui::Menu {
            name: m.label.clone().into(),
            items: m.items.iter().map(menu_item).collect(),
            disabled: false,
        }));
        return Ok(pb::CommandResult::default());
    }
    if windows.pending_prompts.get() >= 8 {
        return Err(core::error(
            pb::ErrorCode::Backpressure,
            "too many pending platform prompts",
        ));
    }
    type PathFuture = std::pin::Pin<
        Box<dyn std::future::Future<Output = Result<Option<Vec<std::path::PathBuf>>, String>>>,
    >;
    let (handler, future): (u64, PathFuture) = match command.kind.as_ref().unwrap() {
        Kind::PromptPaths(p) => {
            let receiver = cx.prompt_for_paths(gpui::PathPromptOptions {
                files: p.files,
                directories: p.directories,
                multiple: p.multiple,
                prompt: (!p.prompt.is_empty()).then(|| p.prompt.clone().into()),
            });
            (
                p.handler_id,
                Box::pin(async move {
                    receiver
                        .await
                        .map_err(|e| e.to_string())?
                        .map_err(|e| e.to_string())
                }),
            )
        }
        Kind::PromptSavePath(p) => {
            let receiver = cx.prompt_for_new_path(
                std::path::Path::new(&p.directory),
                (!p.suggested_name.is_empty()).then_some(p.suggested_name.as_str()),
            );
            (
                p.handler_id,
                Box::pin(async move {
                    receiver
                        .await
                        .map_err(|e| e.to_string())?
                        .map(|p| p.map(|p| vec![p]))
                        .map_err(|e| e.to_string())
                }),
            )
        }
        _ => unreachable!(),
    };
    let pending = windows.pending_prompts.clone();
    pending.set(pending.get() + 1);
    cx.spawn(async move |cx| {
        let result = future.await;
        pending.set(pending.get() - 1);
        let result = match result {
            Ok(paths) => pb::PathResult {
                operation_id: handler,
                cancelled: paths.is_none(),
                paths: paths
                    .unwrap_or_default()
                    .iter()
                    .map(|p| p.to_string_lossy().into_owned())
                    .collect(),
                error: String::new(),
            },
            Err(error) => pb::PathResult {
                operation_id: handler,
                error,
                ..Default::default()
            },
        };
        if hub
            .input(Message::PlatformResult {
                handler_id: handler,
                result,
            })
            .is_err()
        {
            cx.update(|cx| cx.quit());
        }
    })
    .detach();
    Ok(pb::CommandResult {
        operation_id: handler,
        ..Default::default()
    })
}
