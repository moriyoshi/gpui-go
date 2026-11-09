//! Session resource cache and GPUI's in-memory SVG asset source.
use crate::*;
use gpui::{AssetSource, RenderImage};
use std::{borrow::Cow, cell::RefCell, rc::Rc, sync::Mutex};

#[derive(Clone, Default)]
pub(crate) struct AssetBytes(pub Arc<Mutex<HashMap<String, Vec<u8>>>>);
impl AssetSource for AssetBytes {
    fn load(&self, path: &str) -> gpui::Result<Option<Cow<'static, [u8]>>> {
        Ok(self.0.lock().unwrap().get(path).cloned().map(Cow::Owned))
    }
    fn list(&self, path: &str) -> gpui::Result<Vec<SharedString>> {
        Ok(self
            .0
            .lock()
            .unwrap()
            .keys()
            .filter(|p| p.starts_with(path))
            .cloned()
            .map(SharedString::from)
            .collect())
    }
}
#[derive(Clone)]
pub(crate) enum Asset {
    Raster(Arc<RenderImage>),
    Svg(String),
    Font,
}
pub(crate) struct Assets {
    pub items: HashMap<u64, Asset>,
    bytes: AssetBytes,
    sizes: HashMap<u64, usize>,
    resident: usize,
}
pub(crate) type ResourceCache = Rc<RefCell<Assets>>;
impl Assets {
    pub fn new(bytes: AssetBytes) -> Self {
        Self {
            items: HashMap::new(),
            bytes,
            sizes: HashMap::new(),
            resident: 0,
        }
    }
    pub fn update(
        &mut self,
        resource: &pb::Resource,
        load: bool,
        cx: &mut App,
    ) -> Result<(), String> {
        let id = resource.handle.as_ref().unwrap().id;
        if !load {
            if let Some(Asset::Svg(path)) = self.items.remove(&id) {
                self.bytes.0.lock().unwrap().remove(&path);
            }
            let bytes = self.sizes.remove(&id).unwrap_or(0);
            if resource.kind != pb::ResourceKind::Font as i32 {
                self.resident -= bytes;
            }
            return Ok(());
        }
        let (asset, bytes) = match pb::ResourceKind::try_from(resource.kind).unwrap() {
            pb::ResourceKind::Image => {
                let mut reader = image::ImageReader::new(std::io::Cursor::new(&resource.data))
                    .with_guessed_format()
                    .map_err(|e| e.to_string())?;
                let mut limits = image::Limits::default();
                limits.max_image_width = Some(4096);
                limits.max_image_height = Some(4096);
                limits.max_alloc = Some(64 * 1024 * 1024);
                reader.limits(limits);
                let mut buffer = reader.decode().map_err(|e| e.to_string())?.into_rgba8();
                let bytes = buffer.len();
                if self.resident + bytes > 64 * 1024 * 1024 {
                    return Err("decoded resource budget exceeded".into());
                }
                for pixel in buffer.as_chunks_mut::<4>().0 {
                    pixel.swap(0, 2);
                }
                (
                    Asset::Raster(Arc::new(RenderImage::new(vec![image::Frame::new(buffer)]))),
                    bytes,
                )
            }
            pb::ResourceKind::Svg => {
                if self.resident + resource.data.len() > 64 * 1024 * 1024 {
                    return Err("resource budget exceeded".into());
                }
                cx.svg_renderer()
                    .parse_svg(&resource.data)
                    .map_err(|e| e.to_string())?;
                let path = format!("gpui-go-resource/{id}.svg");
                self.bytes
                    .0
                    .lock()
                    .unwrap()
                    .insert(path.clone(), resource.data.clone());
                (Asset::Svg(path), resource.data.len())
            }
            pb::ResourceKind::Font => {
                if self.resident + resource.data.len() > 64 * 1024 * 1024 {
                    return Err("font resource budget exceeded".into());
                }
                cx.text_system()
                    .add_fonts(vec![Cow::Owned(resource.data.clone())])
                    .map_err(|e| e.to_string())?;
                (Asset::Font, resource.data.len())
            }
            _ => unreachable!(),
        };
        self.resident += bytes;
        self.sizes.insert(id, bytes);
        self.items.insert(id, asset);
        Ok(())
    }
}
