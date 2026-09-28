package com.talentflow.vacaciones.infraestructura.persistencia;

import com.talentflow.vacaciones.aplicacion.Transacciones;
import java.util.function.Supplier;
import org.springframework.transaction.support.TransactionTemplate;

public class TransaccionesSpring implements Transacciones {

    private final TransactionTemplate plantilla;

    public TransaccionesSpring(TransactionTemplate plantilla) {
        this.plantilla = plantilla;
    }

    @Override
    public <T> T ejecutar(Supplier<T> bloque) {
        return plantilla.execute(estado -> bloque.get());
    }
}
